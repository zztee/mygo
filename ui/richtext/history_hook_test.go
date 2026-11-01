package richtext

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

// A presentation-only step is skipped before the next input, including
// several undo or redo commands arriving without a frame between them.
func TestHistoryHookBeforeNextInput(t *testing.T) {
	for _, commands := range []bool{false, true} {
		name := "keys"
		if commands {
			name = "commands"
		}
		t.Run(name, func(t *testing.T) {
			d := NewDocument(NewText("base"))
			d.SetSelection(4, 4)
			for _, s := range []string{"x", "[]", "a", "b"} {
				d.BreakUndoCoalescing()
				d.InsertText(s)
			}
			var got []string
			changed := false
			opts := EditorOptions{Label: "Body", OnHistory: func(redo bool) {
				// Both calls take the lock, so the callback also proves it is free.
				text := d.Text().String()
				if text == "basex[]" {
					if redo {
						d.Redo()
					} else {
						d.Undo()
					}
				}
				got = append(got, d.Text().String())
			}}
			tt := ui.NewTester(func(c *ui.Context) {
				if EditorBase(c, d, opts).Fill().Changed() {
					changed = true
				}
			}, 500, 400)
			if err := tt.Click("Body"); err != nil {
				t.Fatal(err)
			}
			changed = false
			tt.Batch(func() {
				for range 3 {
					if commands {
						tt.Command("undo")
					} else {
						tt.Key(ui.Cmd, ui.KeyZ)
					}
				}
			})
			if want := []string{"basex[]a", "basex", "base"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("undo callbacks %q, want %q", got, want)
			}
			if !changed || d.CanUndo() || !d.CanRedo() {
				t.Fatal("undo state or change notice lost")
			}
			if a, c := d.Selection(); a != 4 || c != 4 {
				t.Fatalf("undo selection %d,%d", a, c)
			}
			tt.Command("undo")
			if len(got) != 3 {
				t.Fatalf("exhausted undo invoked the hook: %q", got)
			}
			got = nil
			changed = false
			tt.Batch(func() {
				for range 3 {
					if commands {
						tt.Command("redo")
					} else {
						tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
					}
				}
			})
			if want := []string{"basex", "basex[]a", "basex[]ab"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("redo callbacks %q, want %q", got, want)
			}
			if !changed || !d.CanUndo() || d.CanRedo() {
				t.Fatal("redo state or change notice lost")
			}
			if a, c := d.Selection(); a != 9 || c != 9 {
				t.Fatalf("redo selection %d,%d", a, c)
			}
			// An exhausted history invokes no callback; programmatic operations
			// remain usable by the app without recursively invoking its hook.
			tt.Command("redo")
			d.Undo()
			d.Redo()
			if len(got) != 3 {
				t.Fatalf("extra callbacks %q", got)
			}
			opts.ReadOnly = true
			tt.Frame()
			tt.Command("undo")
			if len(got) != 3 || d.Text().String() != "basex[]ab" {
				t.Fatal("read-only history changed")
			}
		})
	}
}

// An editor with no hook retains the document's normal step boundaries.
func TestHistoryWithoutHookKeepsEachStep(t *testing.T) {
	d := NewDocument(NewText("base"))
	d.SetSelection(4, 4)
	for _, s := range []string{"x", "y", "z"} {
		d.BreakUndoCoalescing()
		d.InsertText(s)
	}
	tt, _ := richTester(t, d)
	tt.Batch(func() { tt.Command("undo"); tt.Command("undo") })
	if got := d.Text().String(); got != "basex" {
		t.Fatalf("undo %q", got)
	}
	tt.Batch(func() { tt.Command("redo"); tt.Command("redo") })
	if got := d.Text().String(); got != "basexyz" {
		t.Fatalf("redo %q", got)
	}
}

// A caller's version observation can precede an external replacement.
// The result comes from the history operation's lock, not that stale
// observation, even when SetText has removed a step that was available.
func TestHistoryResultAfterExternalReplacement(t *testing.T) {
	for _, redo := range []bool{false, true} {
		d := NewDocument(NewText("base"))
		d.SetSelection(4, 4)
		d.InsertText("x")
		if redo {
			d.Undo()
		}
		before := d.Version()
		replace := make(chan struct{})
		replaced := make(chan struct{})
		go func() {
			<-replace
			d.SetText(NewText("external"))
			close(replaced)
		}()
		close(replace)
		<-replaced
		if d.Version() == before {
			t.Fatal("external replacement did not change the observed version")
		}
		if d.undoRedo(redo) {
			t.Fatalf("redo=%v: external replacement counted as a history operation", redo)
		}
		if got := d.Text().String(); got != "external" {
			t.Fatalf("external text %q", got)
		}
		d.InsertText("x")
		if redo {
			d.Undo()
		}
		if !d.undoRedo(redo) {
			t.Fatalf("redo=%v: actual history operation was not reported", redo)
		}
	}
}

// SetText and editor commands may contend for the document, but a copy
// that always has empty history never produces a history notification.
func TestHistoryHookWithConcurrentReplacement(t *testing.T) {
	d := NewDocument(NewText("base"))
	calls := 0
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, d, EditorOptions{Label: "Body", OnHistory: func(bool) { calls++ }}).Fill()
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	tt.Batch(func() {
		for range 32 {
			started, stop, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				d.SetText(NewText("external"))
				close(started)
				for {
					select {
					case <-stop:
						return
					default:
						d.SetText(NewText("external"))
					}
				}
			}()
			<-started
			tt.Command("undo")
			tt.Command("redo")
			close(stop)
			<-done
		}
	})
	if calls != 0 {
		t.Fatalf("empty history notified %d times", calls)
	}
}
