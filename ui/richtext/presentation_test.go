package richtext

import (
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
)

// markedPresentation is a small app policy: one highlight can be shown as
// literal delimiters or as attributed text. It preserves the content's formats.
func markedPresentation(compact bool) func(Presentation) Presentation {
	return func(p Presentation) Presentation {
		s, n := p.Text.String(), p.Text.Len()
		marked := n >= 4 && strings.HasPrefix(s, "==") && strings.HasSuffix(s, "==")
		tagged := n > 0 && p.Text.AttributesAt(0).Tag == "test:highlight"
		switch {
		case compact && marked:
			p.Text = p.Text.Slice(2, n-2).Format(0, n-4, func(a *Attributes) { a.Tag = "test:highlight" })
			p.Anchor, p.Caret = max(0, min(p.Anchor-2, n-4)), max(0, min(p.Caret-2, n-4))
		case !compact && tagged:
			p.Text = p.Text.Format(0, n, func(a *Attributes) { a.Tag = "" }).Append("==", Attributes{}).Insert(0, NewText("=="))
			p.Anchor, p.Caret = p.Anchor+2, p.Caret+2
		}
		if p.Typing != nil {
			p.Typing.Tag = ""
			if compact && (marked || tagged) {
				p.Typing.Tag = "test:highlight"
			}
		}
		return p
	}
}

func assertPresentation(t *testing.T, d *Document, text string, anchor, caret int) {
	t.Helper()
	if got := d.Text().String(); got != text {
		t.Fatalf("text %q, want %q", got, text)
	}
	if a, c := d.Selection(); a != anchor || c != caret {
		t.Fatalf("selection %d..%d, want %d..%d", a, c, anchor, caret)
	}
}

func TestPresentationSwitchAndHistory(t *testing.T) {
	original := NewText("==原abc文==").Format(3, 6, func(a *Attributes) { a.Weight = 700 })
	d := NewDocument(original)
	d.SetSelection(6, 3)
	d.Edit(func(e *Edit) {
		e.ReplaceString(3, 6, "XY", Attributes{Weight: 700})
		e.Select(5, 3)
	})
	d.SetPresentation(markedPresentation(true))
	assertPresentation(t, d, "原XY文", 3, 1)
	if !d.CanUndo() || d.CanRedo() || !d.Text().AttributesAt(1).Bold() {
		t.Fatal("switch lost history or original formatting")
	}
	d.Undo()
	assertPresentation(t, d, "原abc文", 4, 1)
	if d.CanUndo() || !d.CanRedo() || !d.Text().AttributesAt(1).Bold() {
		t.Fatal("presentation added an undo step or lost redo")
	}
	// A pending redo, captured under the old display, takes the newest policy.
	d.SetPresentation(markedPresentation(false))
	assertPresentation(t, d, "==原abc文==", 6, 3)
	d.Redo()
	assertPresentation(t, d, "==原XY文==", 5, 3)
	d.SetPresentation(markedPresentation(true))
	d.Undo()
	assertPresentation(t, d, "原abc文", 4, 1)
	d.Redo()
	assertPresentation(t, d, "原XY文", 3, 1)
}

func TestPresentationViewSelectionAndSameFrameTyping(t *testing.T) {
	for _, afterFrame := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{false: "same-frame", true: "next-frame"}[afterFrame], map[bool]string{false: "forward", true: "reverse"}[reverse]}, "/"), func(t *testing.T) {
				d := NewDocument(NewText("==abcdef=="))
				opts := EditorOptions{}
				tt, _, _ := inputHookTester(t, d, &opts)
				a, c := 3, 6
				if reverse {
					a, c = c, a
				}
				d.SetSelection(a, c)
				tt.Frame()
				d.SetPresentation(markedPresentation(true))
				if afterFrame {
					tt.Frame()
				}
				tt.Type("X")
				assertPresentation(t, d, "aXef", 2, 2)
				tt.Key(ui.Cmd, ui.KeyZ)
				assertPresentation(t, d, "abcdef", a-2, c-2)
				if d.CanUndo() {
					t.Fatal("switch became a history step")
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				assertPresentation(t, d, "aXef", 2, 2)
				// This edit uses the view's restored caret without another Select.
				tt.Type("Y")
				assertPresentation(t, d, "aXYef", 3, 3)
			})
		}
	}
}

func TestPresentationHistoryIsLazyAndBranches(t *testing.T) {
	d := NewDocument(NewText("==x=="))
	for range 150 {
		d.Edit(func(e *Edit) { e.ReplaceString(2, 2, "a", Attributes{}) })
	}
	for range 25 {
		d.Undo()
	}
	calls := 0
	policy := markedPresentation(true)
	d.SetPresentation(func(p Presentation) Presentation { calls++; return policy(p) })
	if calls != 1 || len(d.undo) != 125 || len(d.redo) != 25 {
		t.Fatalf("eager history mapping: calls=%d undo=%d redo=%d", calls, len(d.undo), len(d.redo))
	}
	d.Redo()
	if calls != 2 || d.Text().Len() != 127 {
		t.Fatal("redo did not map once")
	}
	d.SetSelection(0, 0)
	d.Edit(func(e *Edit) { e.ReplaceString(0, 0, "b", Attributes{Tag: "test:highlight"}); e.Select(1, 1) })
	if d.CanRedo() {
		t.Fatal("new edit retained a stale redo branch")
	}
	d.Undo()
	if d.Text().Len() != 127 || strings.HasPrefix(d.Text().String(), "b") {
		t.Fatal("branch undo lost content")
	}
	for range 126 {
		d.Undo()
	}
	if d.Text().String() != "x" || d.CanUndo() {
		t.Fatal("old history was dropped")
	}
	for d.CanRedo() {
		d.Redo()
	}
	if !strings.HasPrefix(d.Text().String(), "b") {
		t.Fatal("branch redo lost content")
	}
}

func TestPresentationExplicitTypingAndOwnership(t *testing.T) {
	d := NewDocument(NewText("**x*"))
	d.SetSelection(4, 4)
	d.SetTypingAttributes(Attributes{Weight: 700, Italic: true})
	d.Edit(func(e *Edit) {
		e.ReplaceString(4, 4, "*", e.TypingAttributes())
		e.Select(5, 5)
		if !e.ConvertInlineMarkdown(5) {
			t.Fatal("atomic conversion did not match")
		}
		e.SetTypingAttributes(Attributes{Underline: true})
	})
	var argument, result *Attributes
	d.SetPresentation(func(p Presentation) Presentation {
		argument = p.Typing
		p.Text = p.Text.Format(0, p.Text.Len(), func(a *Attributes) { a.Tag = "test:highlight" })
		if p.Typing != nil {
			p.Typing.Tag = "test:highlight"
		}
		result = p.Typing
		return p
	})
	if argument == d.typing || result == d.typing {
		t.Fatal("callback aliases document typing")
	}
	result.Code = true
	if d.TypingAttributes().Code {
		t.Fatal("retained callback pointer mutates document")
	}
	d.Undo()
	assertPresentation(t, d, "**x*", 4, 4)
	attrs := d.TypingAttributes()
	if !attrs.Bold() || !attrs.Italic || attrs.Tag != "test:highlight" {
		t.Fatalf("undo typing %+v", attrs)
	}
	d.Redo()
	assertPresentation(t, d, "x", 1, 1)
	if attrs = d.TypingAttributes(); !attrs.Underline || attrs.Tag != "test:highlight" {
		t.Fatalf("redo typing %+v", attrs)
	}
}

func TestPresentationCheckpointAndReset(t *testing.T) {
	d := NewDocument(NewText("==abc=="))
	oldRewind := d.Checkpoint()
	d.SetPresentation(markedPresentation(true))
	if oldRewind(0, d.Text().Len(), 7) {
		t.Fatal("old checkpoint restored an obsolete representation")
	}
	d.SetSelection(1, 1)
	rewind := d.Checkpoint()
	d.Edit(func(e *Edit) { e.ReplaceString(1, 1, "X", Attributes{}); e.Select(2, 2) })
	if !rewind(1, 2, 1) {
		t.Fatal("checkpoint in current representation failed")
	}
	assertPresentation(t, d, "abc", 1, 1)
	if d.CanUndo() || d.CanRedo() {
		t.Fatal("rewind did not preserve history")
	}
	d.SetText(NewText("raw"))
	if d.presentation != nil {
		t.Fatal("SetText retained the old policy")
	}
	d.Edit(func(e *Edit) { e.ReplaceString(3, 3, "!", Attributes{}) })
	d.Undo()
	if d.Text().String() != "raw" {
		t.Fatal("reset document applied stale policy")
	}
}

func TestPresentationIdentityNilAndTypingBoundary(t *testing.T) {
	d := NewDocument(NewText("abc"))
	d.SetSelection(1, 1)
	opts := EditorOptions{}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Type("X")
	version := d.Version()
	d.SetPresentation(func(p Presentation) Presentation { return p })
	if d.Version() != version {
		t.Fatal("identity presentation changed the text version")
	}
	tt.Type("Y")
	d.Undo()
	assertPresentation(t, d, "aXbc", 2, 2)
	d.Undo()
	assertPresentation(t, d, "abc", 1, 1)
	d.SetPresentation(nil)
	d.Redo()
	assertPresentation(t, d, "aXbc", 2, 2)
}

func TestPresentationConcurrentReadsAndSwitches(t *testing.T) {
	d := NewDocument(NewText("==abc=="))
	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if worker%2 == 0 {
					d.SetPresentation(markedPresentation(worker == 0))
				} else {
					_ = d.Text()
					d.Selection()
					d.TypingAttributes()
				}
			}
		}()
	}
	wg.Wait()
	d.SetPresentation(markedPresentation(false))
	if d.Text().String() != "==abc==" || d.CanUndo() {
		t.Fatal("concurrent presentation lost content or added history")
	}
}

func TestPresentationOptionDefersComposition(t *testing.T) {
	for _, selection := range []bool{false, true} {
		for _, finish := range []string{"direct", "cancel", "native-replace"} {
			t.Run(map[bool]string{false: "caret", true: "selection"}[selection]+"/"+finish, func(t *testing.T) {
				d := NewDocument(NewText("==abc=="))
				opts := EditorOptions{Presentation: markedPresentation(false), PresentationKey: "expanded"}
				tt, changed, state := inputHookTester(t, d, &opts)
				a, c := 3, 3
				if selection {
					a, c = 4, 3
				}
				d.SetSelection(a, c)
				tt.Frame()
				tt.Compose("字", 1)
				provisional := d.Text()
				opts.Presentation, opts.PresentationKey = markedPresentation(true), "compact"
				*changed = false
				tt.Frame()
				if !d.Text().Same(provisional) || (*state).compose != "字" || *changed {
					t.Fatal("display switch changed the provisional composition")
				}
				switch finish {
				case "cancel":
					tt.Compose("", 0)
				case "native-replace":
					(*state).EndPreedit()
					tt.Frame()
					if (*state).presentationKey != "expanded" {
						t.Fatal("switch did not wait for pending native replacement")
					}
					(*state).ReplaceText(&ui.TextInputRange{Start: 3, End: 4}, "字")
				default:
					tt.Type("字")
				}
				tt.Frame()
				if (*state).presentationKey != "compact" {
					t.Fatal("completed composition did not apply the requested policy")
				}
				want := "abc"
				// Cancelling a composition leaves an initially replaced selection
				// deleted, as without a presentation policy; that deletion undoes.
				if finish == "cancel" && selection {
					want = "ac"
				}
				if finish != "cancel" {
					want = "a字bc"
					if selection {
						want = "a字c"
					}
				}
				if d.Text().String() != want {
					t.Fatalf("composition result %q, want %q", d.Text().String(), want)
				}
				if finish != "cancel" {
					tt.Key(ui.Cmd, ui.KeyZ)
					assertPresentation(t, d, "abc", a-2, c-2)
					if d.CanUndo() {
						t.Fatal("composition or presentation split undo")
					}
					tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
					if d.Text().String() != want {
						t.Fatal("composition redo lost committed text")
					}
				} else if selection {
					tt.Key(ui.Cmd, ui.KeyZ)
					assertPresentation(t, d, "abc", a-2, c-2)
				}
			})
		}
	}
}

func TestPresentationOptionOnlyMapsOnPolicyChange(t *testing.T) {
	d := NewDocument(NewText("abc"))
	calls := 0
	opts := EditorOptions{Presentation: func(p Presentation) Presentation { calls++; return p }}
	tt, changed, _ := inputHookTester(t, d, &opts)
	*changed = false
	for range 5 {
		tt.Frame()
	}
	if calls != 1 || *changed {
		t.Fatalf("idle maps=%d changed=%v", calls, *changed)
	}
	opts.PresentationKey = "next"
	tt.Frame()
	if calls != 2 || *changed {
		t.Fatalf("switch maps=%d changed=%v", calls, *changed)
	}
	opts.Presentation = nil
	tt.Frame()
	if d.presentation != nil {
		t.Fatal("removing option left history mapping active")
	}
}

func TestPresentationNativeReplacementRedoCaret(t *testing.T) {
	// Native replacement records the final collapsed caret even without a
	// presentation policy; the same snapshot is mapped if one is later enabled.
	d := NewDocument(NewText("abcdef"))
	opts := EditorOptions{}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(4, 1)
	tt.Frame()
	tt.Type("X")
	tt.Key(ui.Cmd, ui.KeyZ)
	assertPresentation(t, d, "abcdef", 4, 1)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	assertPresentation(t, d, "aXef", 2, 2)
	tt.Type("Y")
	assertPresentation(t, d, "aXYef", 3, 3)
}

func TestPresentationRemovingPolicyRefreshesWholeHistory(t *testing.T) {
	d := NewDocument(NewText("==one\ntwo\nthree=="))
	d.SetSelection(10, 10)
	d.Edit(func(e *Edit) { e.ReplaceString(10, 10, "X\n", Attributes{}); e.Select(12, 12) })
	opts := EditorOptions{}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetPresentation(markedPresentation(true))
	tt.Frame()
	d.SetPresentation(nil)
	version, oldLength := d.Version(), d.Text().Len()
	d.Undo()
	c, ok := d.since(version)
	if !ok || c.start != 0 || c.oldEnd != oldLength || c.newEnd != d.Text().Len() {
		t.Fatalf("restoring raw history after policy removal did not refresh whole text: %+v", c)
	}
	tt.Frame()
	assertPresentation(t, d, "==one\ntwo\nthree==", 10, 10)
	if !(*state).shown.Equal(d.Text()) {
		t.Fatal("view kept stale represented text")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	assertPresentation(t, d, "==one\ntwo\nX\nthree==", 12, 12)
	tt.Type("Y")
	assertPresentation(t, d, "==one\ntwo\nX\nYthree==", 13, 13)
}

func TestPresentationOptionReappliedAfterSetText(t *testing.T) {
	d := NewDocument(NewText("==old=="))
	opts := EditorOptions{Presentation: markedPresentation(true), PresentationKey: "compact"}
	tt, changed, _ := inputHookTester(t, d, &opts)
	if d.Text().String() != "old" {
		t.Fatal("initial policy not applied")
	}
	d.SetText(NewText("==new文=="))
	*changed = false
	tt.Frame()
	if d.Text().String() != "new文" || d.presentation == nil || d.CanUndo() || *changed {
		t.Fatal("same document and key did not reapply presentation after SetText")
	}
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Type("X")
	tt.Key(ui.Cmd, ui.KeyZ)
	assertPresentation(t, d, "new文", 1, 1)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	assertPresentation(t, d, "nXew文", 2, 2)
}

func TestPresentationOptionAbandonsPendingOnBlur(t *testing.T) {
	d := NewDocument(NewText("==abc=="))
	opts := EditorOptions{Label: "Body", Presentation: markedPresentation(false), PresentationKey: "expanded"}
	var state *view
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			e := EditorBase(c, d, opts).Height(220).FillWidth()
			state = ui.Local(e, "richtext", func() view { return view{} })
			ui.Button(c, "Other control")
		})
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	d.SetSelection(3, 3)
	tt.Frame()
	tt.Compose("字", 1)
	state.EndPreedit()
	if !state.composePending {
		t.Fatal("fixture has no pending native replacement")
	}
	opts.Presentation, opts.PresentationKey = markedPresentation(true), "compact"
	tt.Frame()
	if state.presentationKey != "expanded" {
		t.Fatal("switch did not wait while native replacement remained active")
	}
	if err := tt.Click("Other control"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if state.composePending || state.presentationKey != "compact" || d.Text().String() != "a字bc" {
		t.Fatal("blur left a pending replacement blocking presentation")
	}
	// No new input into the editor is needed for further settings changes.
	opts.Presentation, opts.PresentationKey = markedPresentation(false), "expanded-again"
	tt.Frame()
	if d.Text().String() != "==a字bc==" {
		t.Fatal("blurred editor still blocked policy changes")
	}
}

func TestPresentationBatchedHistoryAndTyping(t *testing.T) {
	for _, commands := range []bool{false, true} {
		t.Run(map[bool]string{false: "keys", true: "commands"}[commands], func(t *testing.T) {
			d := NewDocument(NewText("==abcdef=="))
			opts := EditorOptions{Presentation: markedPresentation(true), PresentationKey: "compact"}
			tt, _, _ := inputHookTester(t, d, &opts)
			d.SetSelection(4, 1)
			tt.Frame()
			tt.Type("X")
			tt.Batch(func() {
				if commands {
					tt.Command("undo")
					tt.Command("redo")
				} else {
					tt.Key(ui.Cmd, ui.KeyZ)
					tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				}
				tt.Type("Y")
			})
			assertPresentation(t, d, "aXYef", 3, 3)
			tt.Batch(func() {
				if commands {
					tt.Command("undo")
					tt.Command("undo")
				} else {
					tt.Key(ui.Cmd, ui.KeyZ)
					tt.Key(ui.Cmd, ui.KeyZ)
				}
			})
			assertPresentation(t, d, "abcdef", 4, 1)
			if d.CanUndo() {
				t.Fatal("policy added a semantic undo step")
			}
		})
	}
}

func TestPresentationSharedDocumentKeepsPolicyAndCheckpoint(t *testing.T) {
	d := NewDocument(NewText("==abc=="))
	calls := 0
	policy := markedPresentation(true)
	opts := EditorOptions{Presentation: func(p Presentation) Presentation { calls++; return policy(p) }, PresentationKey: "compact"}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			for _, label := range []string{"First body", "Second body"} {
				options := opts
				options.Label = label
				EditorBase(c, d, options).Height(180).FillWidth()
			}
		})
	}, 500, 400)
	tt.Frame()
	if calls != 1 {
		t.Fatalf("shared document installed policy %d times", calls)
	}
	rewind := d.Checkpoint()
	for range 5 {
		tt.Frame()
	}
	if calls != 1 || !rewind(0, d.Text().Len(), d.Text().Len()) {
		t.Fatal("idle shared editors reinstalled policy or invalidated checkpoint")
	}
	d.SetText(NewText("==new=="))
	tt.Frame()
	if calls != 2 || d.Text().String() != "new" {
		t.Fatalf("shared reset maps=%d text=%q", calls, d.Text().String())
	}
	if err := tt.Click("First body"); err != nil {
		t.Fatal(err)
	}
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Type("X")
	tt.Frame()
	tt.Type("Y")
	tt.Key(ui.Cmd, ui.KeyZ)
	assertPresentation(t, d, "new", 1, 1)
	if d.CanUndo() {
		t.Fatal("idle shared editor broke typing coalescing")
	}
}

func TestPresentationOptionEnabledAfterPendingBlur(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "caret", true: "reverse-selection"}[selected], func(t *testing.T) {
			d := NewDocument(NewText("==abc=="))
			opts := EditorOptions{Label: "Body"}
			var state *view
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					e := EditorBase(c, d, opts).Height(220).FillWidth()
					state = ui.Local(e, "richtext", func() view { return view{} })
					ui.Button(c, "Other control")
				})
			}, 500, 400)
			if err := tt.Click("Body"); err != nil {
				t.Fatal(err)
			}
			a, c := 3, 3
			if selected {
				a = 4
			}
			d.SetSelection(a, c)
			tt.Frame()
			tt.Compose("字", 1)
			state.EndPreedit()
			if err := tt.Click("Other control"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			want := "a字bc"
			if selected {
				want = "a字c"
			}
			if state.focused || d.Text().String() != "=="+want+"==" {
				t.Fatal("fixture did not blur the pending editor")
			}
			opts.Presentation, opts.PresentationKey = markedPresentation(true), "compact"
			tt.Frame()
			if state.composePending || state.presentationKey != "compact" || d.Text().String() != want {
				t.Fatal("enabling policy after blur left pending replacement blocked")
			}
			if err := tt.Click("Body"); err != nil {
				t.Fatal(err)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			assertPresentation(t, d, "abc", a-2, c-2)
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			assertPresentation(t, d, want, 2, 2)
		})
	}
}
