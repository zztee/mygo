package richtext

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func chooseEdit(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if err := tt.ChooseMenuItem(name); err != nil {
		t.Fatal(err)
	}
}

func TestRichTextDefaultMenu(t *testing.T) {
	d := NewDocument(ParseMarkdown("hello world"))
	tt, _ := richTester(t, d)
	d.SetSelection(0, 5)
	tt.Frame()
	original, version := d.Text().String(), d.Version()
	tt.RightClickAt(250, 100)
	for _, name := range []string{"Cut", "Copy", "Paste", "Select All"} {
		if !slices.Contains(tt.Menu(), name) {
			t.Fatalf("missing %s: %q", name, tt.Menu())
		}
	}
	tt.CloseMenu()
	if a, b := d.Selection(); a != 0 || b != 5 || d.Version() != version || d.Text().String() != original {
		t.Fatal("cancelling changed document")
	}
	tt.Key(ui.Shift, ui.KeyF10)
	chooseEdit(t, tt, "Copy")
	if tt.Clipboard() != "hello" || d.Version() != version {
		t.Fatalf("copy %q, version %d", tt.Clipboard(), d.Version())
	}
	tt.Key(ui.Shift, ui.KeyF10)
	chooseEdit(t, tt, "Cut")
	if d.Text().String() != " world" {
		t.Fatalf("cut: %q", d.Text().String())
	}
	tt.Command("undo")
	if d.Text().String() != original {
		t.Fatal("cut undo")
	}
	tt.Command("redo")
	if d.Text().String() != " world" {
		t.Fatal("cut redo")
	}
	tt.SetClipboard("new")
	d.SetSelection(0, 0)
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyF10)
	chooseEdit(t, tt, "Paste")
	if d.Text().String() != "new world" {
		t.Fatalf("paste: %q", d.Text().String())
	}
	tt.Key(ui.Shift, ui.KeyF10)
	chooseEdit(t, tt, "Select All")
	if a, b := d.Selection(); a != 0 || b != d.Text().Len() {
		t.Fatalf("select all %d %d", a, b)
	}
}

func TestRichTextMenuEmptyReadOnlyAndChanges(t *testing.T) {
	d := NewDocument(Text{})
	opts := EditorOptions{}
	tt := ui.NewTester(func(c *ui.Context) { Editor(c, d, opts).Fill() }, 400, 250)
	tt.Frame()
	tt.RightClickAt(100, 100)
	for _, name := range []string{"Copy", "Cut", "Select All"} {
		if err := tt.ChooseMenuItem(name); err == nil {
			t.Fatalf("empty enabled %s", name)
		}
	}
	tt.CloseMenu()
	d = NewDocument(ParseMarkdown("hello"))
	opts.ReadOnly = true
	d.SetSelection(0, 5)
	tt.Frame()
	tt.RightClickAt(100, 100)
	if !slices.Equal(tt.Menu(), []string{"Copy", "-", "Select All"}) {
		t.Fatalf("readonly %q", tt.Menu())
	}
	chooseEdit(t, tt, "Copy")
	if tt.Clipboard() != "hello" {
		t.Fatalf("readonly copy %q", tt.Clipboard())
	}
	opts.ReadOnly = false
	tt.Frame()
	tt.RightClickAt(100, 100)
	opts.ReadOnly = true
	tt.Frame()
	chooseEdit(t, tt, "Cut")
	if d.Text().String() != "hello" || d.CanUndo() {
		t.Fatal("stale write edited readonly document")
	}
	opts.ReadOnly = false
	tt.Frame()
	tt.RightClickAt(100, 100)
	old := d
	d = NewDocument(ParseMarkdown("other"))
	d.SetSelection(0, 5)
	tt.Frame()
	chooseEdit(t, tt, "Cut")
	if old.Text().String() != "hello" || d.Text().String() != "other" || old.CanUndo() || d.CanUndo() {
		t.Fatal("stale menu edited switched document")
	}
}

func TestRichTextDefaultMenuOnAttachmentAndBelow(t *testing.T) {
	t.Run("attachment", func(t *testing.T) {
		take := true
		tt, d, scroll, _, _ := attachmentTester(t, 100, 50, &take)
		d.SetSelection(0, 2)
		tt.Frame()
		box, _ := scroll.AttachmentBox(2)
		tt.RightClickAt(box.X+20, box.Y+10)
		chooseEdit(t, tt, "Copy")
		if tt.Clipboard() != "ab" {
			t.Fatalf("attachment copy %q", tt.Clipboard())
		}
	})
	t.Run("below", func(t *testing.T) {
		tt, d, _, b, _, _ := belowPressTester(t)
		d.SetSelection(0, 3)
		tt.Frame()
		r := b.drawn[len(b.drawn)-1]
		tt.RightClickAt(r.X+10, r.Y+10)
		chooseEdit(t, tt, "Copy")
		if tt.Clipboard() != "one" {
			t.Fatalf("below copy %q", tt.Clipboard())
		}
	})
	t.Run("long", func(t *testing.T) {
		d := NewDocument(ParseMarkdown(strings.Repeat("long text\n\n", 1000)))
		tt, _ := richTester(t, d)
		tt.Key(ui.Shift, ui.KeyF10)
		chooseEdit(t, tt, "Select All")
		if a, b := d.Selection(); a != 0 || b != d.Text().Len() {
			t.Fatalf("long selection %d %d", a, b)
		}
	})
}
