package richtext

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestReturnHookBeforeNextInput(t *testing.T) {
	for _, replace := range []bool{false, true} {
		d := NewDocument(NewText("before $x$ after"))
		calls := 0
		changed := false
		tt := ui.NewTester(func(c *ui.Context) {
			if EditorBase(c, d, EditorOptions{Label: "Body", OnReturn: func() bool {
				calls++
				if replace {
					d.SetText(NewText("replaced"))
					d.SetSelection(8, 8)
				} else {
					d.SetSelection(10, 10)
				}
				return true
			}}).Fill().Changed() {
				changed = true
			}
		}, 500, 400)
		if err := tt.Click("Body"); err != nil {
			t.Fatal(err)
		}
		d.SetSelection(7, 7)
		tt.Frame()
		changed = false
		tt.Batch(func() { tt.Key(0, ui.KeyEnter); tt.Type("z") })
		want := "before $x$z after"
		if replace {
			want = "replacedz"
		}
		if got := d.Text().String(); got != want {
			t.Fatalf("replace=%v text %q want %q", replace, got, want)
		}
		if calls != 1 || !changed {
			t.Fatalf("calls=%d changed=%v", calls, changed)
		}
		_, caret := d.Selection()
		if replace && caret != 9 || !replace && caret != 11 {
			t.Fatalf("caret %d", caret)
		}
	}
}

func TestReturnHookEditsThenContinuesInOrder(t *testing.T) {
	d := NewDocument(NewText("ab"))
	calls := 0
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, d, EditorOptions{Label: "Body", OnReturn: func() bool {
			calls++
			// The callback runs without the document lock and sees the last event's selection.
			d.InsertText("\n")
			return true
		}}).Fill()
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Batch(func() { tt.Key(0, ui.KeyEnter); tt.Type("x"); tt.Key(0, ui.KeyEnter); tt.Type("y") })
	if got := d.Text().String(); got != "a\nx\nyb" || calls != 2 {
		t.Fatalf("text=%q calls=%d", got, calls)
	}
	d.Undo()
	tt.Frame()
	if got := d.Text().String(); got != "a\nx\nb" {
		t.Fatalf("undo text %q", got)
	}
}

func TestReturnHookFallsThrough(t *testing.T) {
	for _, hook := range []bool{false, true} {
		d := NewDocument(NewText("abc"))
		calls := 0
		opts := EditorOptions{Label: "Body"}
		if hook {
			opts.OnReturn = func() bool { calls++; d.SetSelection(1, 1); return false }
		}
		tt := ui.NewTester(func(c *ui.Context) { EditorBase(c, d, opts).Fill() }, 500, 400)
		if err := tt.Click("Body"); err != nil {
			t.Fatal(err)
		}
		d.SetSelection(2, 2)
		tt.Frame()
		tt.Batch(func() { tt.Key(0, ui.KeyEnter); tt.Type("x") })
		want := "ab\nxc"
		if hook {
			want = "a\nxbc"
		}
		if got := d.Text().String(); got != want {
			t.Fatalf("hook=%v text %q want %q", hook, got, want)
		}
		if hook && calls != 1 {
			t.Fatalf("calls %d", calls)
		}
	}
}

func TestReturnHookPreservesSelectionAndUndo(t *testing.T) {
	d := NewDocument(NewText("abcde"))
	changed := false
	tt := ui.NewTester(func(c *ui.Context) {
		if EditorBase(c, d, EditorOptions{Label: "Body", OnReturn: func() bool { d.InsertText("!"); return true }}).Fill().Changed() {
			changed = true
		}
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	d.SetSelection(1, 4)
	tt.Frame()
	changed = false
	tt.Key(0, ui.KeyEnter)
	if got := d.Text().String(); got != "a!e" || !changed {
		t.Fatalf("text %q changed %v", got, changed)
	}
	d.Undo()
	tt.Frame()
	if got := d.Text().String(); got != "abcde" {
		t.Fatalf("undo %q", got)
	}
	d.Redo()
	tt.Frame()
	if got := d.Text().String(); got != "a!e" {
		t.Fatalf("redo %q", got)
	}
}

func TestReturnHookDoesNotTakeModifiedOrReadOnlyInput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mods     ui.Modifiers
		readonly bool
		want     string
	}{
		{"shift", ui.Shift, false, "a" + string(LineBreak) + "b"},
		{"command", ui.Cmd, false, "ab"},
		{"alt", ui.Alt, false, "ab"},
		{"readonly", 0, true, "ab"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDocument(NewText("ab"))
			calls := 0
			tt := ui.NewTester(func(c *ui.Context) {
				EditorBase(c, d, EditorOptions{Label: "Body", ReadOnly: tc.readonly, OnReturn: func() bool { calls++; return true }}).Fill()
			}, 500, 400)
			if err := tt.Click("Body"); err != nil {
				t.Fatal(err)
			}
			d.SetSelection(1, 1)
			tt.Frame()
			tt.Key(tc.mods, ui.KeyEnter)
			if got := d.Text().String(); got != tc.want || calls != 0 {
				t.Fatalf("text %q calls %d", got, calls)
			}
		})
	}
}

func TestReturnHookCanConsumeWithoutEditing(t *testing.T) {
	d := NewDocument(NewText("abc"))
	calls := 0
	changed := false
	tt := ui.NewTester(func(c *ui.Context) {
		if EditorBase(c, d, EditorOptions{Label: "Body", OnReturn: func() bool { calls++; return true }}).Fill().Changed() {
			changed = true
		}
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	d.SetSelection(1, 1)
	tt.Frame()
	changed = false
	tt.Key(0, ui.KeyEnter)
	if got := d.Text().String(); got != "abc" || calls != 1 || changed || d.CanUndo() {
		t.Fatalf("text=%q calls=%d changed=%v undo=%v", got, calls, changed, d.CanUndo())
	}
}
