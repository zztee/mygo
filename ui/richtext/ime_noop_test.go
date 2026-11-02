package richtext

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestEmptyIMECommitPreservesHistory(t *testing.T) {
	for _, history := range []string{"fresh", "undo", "redo"} {
		for _, commit := range []string{"implicit", "explicit", "marked"} {
			t.Run(history+"/"+commit, func(t *testing.T) {
				d := NewDocument(ParseMarkdown("**a**"))
				opts := EditorOptions{}
				tt, changed, state := inputHookTester(t, d, &opts)
				d.SetSelection(1, 1)
				tt.Frame()
				if history != "fresh" {
					tt.Type("b")
				}
				if history == "redo" {
					tt.Key(ui.Cmd, ui.KeyZ)
				}
				before := d.Text().Markdown()
				undo, redo := d.CanUndo(), d.CanRedo()
				version := d.version
				*changed = false
				switch commit {
				case "implicit":
					(*state).ReplaceText(nil, "")
				case "explicit":
					r := (*state).Selection().Range
					(*state).ReplaceText(&r, "")
				case "marked":
					tt.Compose("中", 1)
					(*state).ReplaceText(nil, "")
				}
				tt.Frame()
				if got := d.Text().Markdown(); got != before || d.version != version || *changed || d.CanUndo() != undo || d.CanRedo() != redo {
					t.Fatalf("empty commit: text %q want %q, version %d want %d, changed %v, undo %v want %v, redo %v want %v", got, before, d.version, version, *changed, d.CanUndo(), undo, d.CanRedo(), redo)
				}
				if history == "undo" {
					tt.Key(ui.Cmd, ui.KeyZ)
					if got := d.Text().Markdown(); got != "**a**" {
						t.Fatalf("extra undo step: %q", got)
					}
				}
				if history == "redo" {
					tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
					if got := d.Text().Markdown(); got != "**ab**" {
						t.Fatalf("lost redo: %q", got)
					}
				}
			})
		}
	}
}

func TestEmptyIMECommitStillDeletesSelection(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "implicit", true: "explicit"}[explicit], func(t *testing.T) {
			d := NewDocument(ParseMarkdown("a **word** z"))
			opts := EditorOptions{}
			tt, _, state := inputHookTester(t, d, &opts)
			d.SetSelection(6, 2)
			tt.Frame()
			var r *ui.TextInputRange
			if explicit {
				selected := (*state).Selection().Range
				r = &selected
			}
			(*state).ReplaceText(r, "")
			tt.Frame()
			if got := d.Text().String(); got != "a  z" {
				t.Fatalf("deletion: %q", got)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if got := d.Text().Markdown(); got != "a **word** z" {
				t.Fatalf("undo deletion: %q", got)
			}
			if a, c := d.Selection(); a != 6 || c != 2 {
				t.Fatalf("selection %d,%d", a, c)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if got := d.Text().String(); got != "a  z" {
				t.Fatalf("redo deletion: %q", got)
			}
		})
	}
}

func TestEmptyIMEExplicitRangeMovesCaretWithoutEditing(t *testing.T) {
	d := NewDocument(NewText("abc"))
	opts := EditorOptions{}
	tt, changed, state := inputHookTester(t, d, &opts)
	d.SetSelection(3, 3)
	tt.Frame()
	*changed = false
	(*state).ReplaceText(&ui.TextInputRange{Start: 1, End: 1}, "")
	tt.Frame()
	if a, c := d.Selection(); a != 1 || c != 1 {
		t.Fatalf("explicit caret %d,%d", a, c)
	}
	if d.Text().String() != "abc" || *changed || d.CanUndo() {
		t.Fatal("empty replacement edited document")
	}
	tt.Type("x")
	if got := d.Text().String(); got != "axbc" {
		t.Fatalf("next input: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().String(); got != "abc" {
		t.Fatalf("undo next input: %q", got)
	}
}
