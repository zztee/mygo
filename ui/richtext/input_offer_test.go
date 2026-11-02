package richtext

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestInputOfferSameBatchSelection(t *testing.T) {
	d := NewDocument(ParseMarkdown("**alpha** beta\n\nuntouched `text`"))
	before := d.Text().Markdown()
	calls := 0
	opts := EditorOptions{OnInput: func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown || ev.Mods != ui.Cmd || ev.Key != ui.KeyD {
			return false
		}
		calls++
		d.SetSelection(0, 5)
		return true
	}}
	tt, changed, _ := inputHookTester(t, d, &opts)
	d.SetSelection(2, 2)
	tt.Frame()
	*changed = false
	tt.Batch(func() { tt.Key(ui.Cmd, ui.KeyD); tt.Type("x") })
	if got := d.Text().Markdown(); got != "**x** beta\n\nuntouched `text`" || calls != 1 || !*changed {
		t.Fatalf("text %q calls %d changed %v", got, calls, *changed)
	}
	tt.Command("undo")
	if d.Text().Markdown() != before {
		t.Fatalf("undo %q", d.Text().Markdown())
	}
	tt.Command("redo")
	if d.Text().Markdown() != "**x** beta\n\nuntouched `text`" {
		t.Fatalf("redo %q", d.Text().Markdown())
	}
}

func TestInputOfferSameBatchDeleteHistory(t *testing.T) {
	d := NewDocument(ParseMarkdown("**alpha** beta\n\nuntouched `text`"))
	before := d.Text().Markdown()
	opts := EditorOptions{OnInput: func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown || ev.Key != ui.KeyD || ev.Mods != ui.Cmd|ui.Shift {
			return false
		}
		d.Edit(func(e *Edit) { e.Replace(0, 5, Text{}); e.Select(0, 0) })
		return true
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	tt.Batch(func() { tt.Key(ui.Cmd|ui.Shift, ui.KeyD); tt.Type("fresh") })
	if got := d.Text().String(); got != "fresh beta\nuntouched text" {
		t.Fatalf("text %q", got)
	}
	tt.Command("undo")
	if got := d.Text().Markdown(); got != "&#32;beta\n\nuntouched `text`" {
		t.Fatalf("typing undo %q", got)
	}
	tt.Command("undo")
	if d.Text().Markdown() != before {
		t.Fatalf("command undo %q", d.Text().Markdown())
	}
}

func TestInputOfferPublishesSelectionAndDeclinesOnce(t *testing.T) {
	d := NewDocument(NewText("abc"))
	var selections [][2]int
	calls := 0
	opts := EditorOptions{OnInput: func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyD {
			calls++
			a, b := d.Selection()
			selections = append(selections, [2]int{a, b})
			d.SetSelection(0, 0)
		}
		return false
	}}
	tt, changed, _ := inputHookTester(t, d, &opts)
	d.SetSelection(3, 3)
	tt.Frame()
	*changed = false
	tt.Batch(func() { tt.Key(0, ui.KeyLeft); tt.Key(ui.Cmd, ui.KeyD); tt.Type("x") })
	if calls != 1 || len(selections) != 1 || selections[0] != [2]int{2, 2} || d.Text().String() != "xabc" {
		t.Fatalf("calls %d selections %v text %q", calls, selections, d.Text().String())
	}
}

func TestInputOfferNativeCopyAndSelectionOnly(t *testing.T) {
	d := NewDocument(NewText("alpha"))
	version := d.Version()
	var services ui.Services
	copies := 0
	opts := EditorOptions{OnInput: func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputCommand && ev.Text == "copy" {
			copies++
			services.WriteClipboard("| selected row |\n")
			return true
		}
		if ev.Kind == ui.InputKeyDown && ev.Mods == ui.Cmd && ev.Key == ui.KeyD {
			d.SetSelection(0, 5)
			return true
		}
		return false
	}}
	tt := ui.NewTester(func(c *ui.Context) { services = c.Services(); EditorBase(c, d, opts).Fill() }, 500, 400)
	// An unlabeled editor still exposes its default label.
	if err := tt.Click("Rich text"); err != nil {
		t.Fatal(err)
	}
	tt.Command("copy")
	if copies != 1 || tt.Clipboard() != "| selected row |\n" {
		t.Fatalf("copies %d clipboard %q", copies, tt.Clipboard())
	}
	tt.Key(ui.Cmd, ui.KeyD)
	if d.Version() != version || d.CanUndo() {
		t.Fatal("copy/selection changed history")
	}
	tt.Command("cut")
	if d.Text().String() != "" || tt.Clipboard() != "alpha" {
		t.Fatalf("default cut text %q clipboard %q", d.Text().String(), tt.Clipboard())
	}
}

func TestInputOfferCompositionAndReadOnly(t *testing.T) {
	d := NewDocument(NewText(""))
	keyCalls := 0
	opts := EditorOptions{OnInput: func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputKeyDown {
			keyCalls++
		}
		if ev.Kind == ui.InputText || ev.Kind == ui.InputCompose {
			t.Fatal("composition/typing offered")
		}
		return false
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	tt.Compose("字", 1)
	tt.Key(ui.Cmd, ui.KeyD)
	if keyCalls != 0 {
		t.Fatalf("composing key calls %d", keyCalls)
	}
	tt.Type("字")
	tt.Key(0, ui.KeyLeft)
	if keyCalls != 1 || d.Text().String() != "字" {
		t.Fatalf("committed text %q key calls %d", d.Text().String(), keyCalls)
	}
	opts.ReadOnly = true
	tt.Frame()
	tt.Batch(func() { tt.Key(0, ui.KeyBackspace); tt.Type("lost") })
	if d.Text().String() != "字" {
		t.Fatal("read-only default edited text")
	}
}
