package richtext

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func inputHookTester(t *testing.T, d *Document, opts *EditorOptions) (*ui.Tester, *bool, **view) {
	t.Helper()
	changed := new(bool)
	var state *view
	opts.Label = "Body"
	tt := ui.NewTester(func(c *ui.Context) {
		e := EditorBase(c, d, *opts).Fill()
		state = ui.Local(e, "richtext", func() view { return view{} })
		if e.Changed() {
			*changed = true
		}
	}, 500, 400)
	if err := tt.Click("Body"); err != nil {
		t.Fatal(err)
	}
	return tt, changed, &state
}

// hookPair is an app's minimal pairing policy. Inserting around the
// selection keeps its existing attributed text instead of rebuilding it.
func hookPair(e *Edit, s string) bool {
	a, c := e.Selection()
	a, c = min(a, c), max(a, c)
	switch s {
	case "(", "（":
		close := ")"
		if s == "（" {
			close = "）"
		}
		attrs := e.TypingAttributes()
		if a != c {
			attrs = Attributes{}
		}
		e.ReplaceString(c, c, close, attrs)
		e.ReplaceString(a, a, s, attrs)
		e.Select(a+1, c+1)
		return true
	case ")":
		if a == c && c < e.Text().Len() && e.Text().Slice(c, c+1).String() == ")" {
			e.Select(c+1, c+1)
			return true
		}
	}
	return false
}

func TestTextInputHookTypingSelectionHistory(t *testing.T) {
	d := NewDocument(ParseMarkdown("a **word** z"))
	opts := EditorOptions{OnTextInput: hookPair}
	tt, changed, _ := inputHookTester(t, d, &opts)
	d.SetSelection(6, 2)
	tt.Frame()
	*changed = false
	tt.Type("(")
	if got := d.Text().Markdown(); got != "a (**word**) z" || !*changed {
		t.Fatalf("wrapped %q changed %v", got, *changed)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().Markdown(); got != "a **word** z" {
		t.Fatalf("undo %q", got)
	}
	if a, c := d.Selection(); a != 6 || c != 2 {
		t.Fatalf("undo selection %d %d", a, c)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if got := d.Text().Markdown(); got != "a (**word**) z" {
		t.Fatalf("redo %q", got)
	}
}

func TestTextInputHookSameFrameAndDecline(t *testing.T) {
	d := NewDocument(NewText(""))
	opts := EditorOptions{OnTextInput: hookPair}
	tt, _, state := inputHookTester(t, d, &opts)
	tt.Batch(func() { tt.Type("("); tt.Type("x"); tt.Type(")"); tt.Type("z") })
	if got := d.Text().String(); got != "(x)z" {
		t.Fatalf("same frame %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().String(); got != "(x)" {
		t.Fatalf("undo final typing %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().String(); got != "()" {
		t.Fatalf("undo inner typing %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().String(); got != "" {
		t.Fatalf("undo pair %q", got)
	}
	tt.Type("a")
	v := *state
	mode, coalesce, docCoalesce, lastEdit, steps := v.mode, v.coalesce, d.coalesce, d.lastEdit, len(d.undo)
	if handled, consumed := v.textInputHook("b", false); handled || consumed || v.mode != mode || v.coalesce != coalesce || d.coalesce != docCoalesce || d.lastEdit != lastEdit || len(d.undo) != steps {
		t.Fatal("declined hook changed the typing coalescing state")
	}
}

func TestTextInputHookCompose(t *testing.T) {
	for _, commit := range []string{"type", "unmark", "range", "end-preedit"} {
		for _, handled := range []bool{false, true} {
			t.Run(commit+map[bool]string{false: "-declined", true: "-handled"}[handled], func(t *testing.T) {
				d := NewDocument(ParseMarkdown("😀 **选区** z"))
				calls := 0
				opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
					calls++
					if a, c := e.Selection(); a != 4 || c != 2 {
						t.Fatalf("committed selection %d %d", a, c)
					}
					if got := e.Text().Markdown(); got != "😀 **选区** z" {
						t.Fatalf("commit lost original text: %q", got)
					}
					return handled && hookPair(e, s)
				}}
				tt, _, state := inputHookTester(t, d, &opts)
				d.SetSelection(4, 2)
				tt.Frame()
				tt.Compose("quan", 4)
				tt.Compose("（", 1)
				if calls != 0 {
					t.Fatal("hook ran during composition")
				}
				switch commit {
				case "type":
					tt.Type("（")
				case "unmark":
					(*state).UnmarkText()
					tt.Frame()
				case "end-preedit":
					(*state).EndPreedit()
					if calls != 0 {
						t.Fatal("hook ran before final replacement")
					}
					(*state).ReplaceText(&ui.TextInputRange{Start: 3, End: 4}, "（")
					tt.Frame()
				case "range":
					v := *state
					v.ReplaceText(&ui.TextInputRange{Start: 3, End: 4}, "（")
					tt.Frame()
				}
				want := "😀 **（** z"
				if handled {
					want = "😀 （**选区**） z"
				}
				if got := d.Text().Markdown(); got != want || calls != 1 {
					t.Fatalf("commit %q want %q calls %d", got, want, calls)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				if got := d.Text().Markdown(); got != "😀 **选区** z" {
					t.Fatalf("IME undo %q", got)
				}
				if a, c := d.Selection(); a != 4 || c != 2 {
					t.Fatalf("IME undo selection %d %d", a, c)
				}
			})
		}
	}
}

func TestBackspaceInputHookAndModifiers(t *testing.T) {
	d := NewDocument(NewText("a()"))
	calls := 0
	opts := EditorOptions{OnBackspace: func(e *Edit) bool {
		calls++
		a, c := e.Selection()
		if a == c && c > 0 && c < e.Text().Len() && e.Text().Slice(c-1, c+1).String() == "()" {
			e.ReplaceString(c-1, c+1, "", Attributes{})
			e.Select(c-1, c-1)
			return true
		}
		return false
	}}
	tt, changed, _ := inputHookTester(t, d, &opts)
	d.SetSelection(2, 2)
	tt.Frame()
	*changed = false
	tt.Key(0, ui.KeyBackspace)
	if got := d.Text().String(); got != "a" || calls != 1 || !*changed {
		t.Fatalf("pair backspace %q calls %d changed %v", got, calls, *changed)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if got := d.Text().String(); got != "a()" {
		t.Fatalf("undo deletion %q", got)
	}
	if a, c := d.Selection(); a != 2 || c != 2 {
		t.Fatalf("undo caret %d %d", a, c)
	}
	tt.Key(ui.Alt, ui.KeyBackspace)
	if calls != 1 {
		t.Fatal("modified backspace called hook")
	}
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	if calls != 2 {
		t.Fatalf("ordinary backspace calls %d", calls)
	}
}

func TestInputHooksReadOnlyPasteAndExplicitReplacement(t *testing.T) {
	d := NewDocument(NewText("word"))
	calls := 0
	opts := EditorOptions{
		OnTextInput: func(*Edit, string) bool { calls++; return false },
		OnBackspace: func(*Edit) bool { calls++; return false },
	}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetSelection(0, 4)
	tt.Frame()
	tt.Command("copy")
	d.SetSelection(4, 4)
	tt.Frame()
	tt.Command("paste")
	(*state).ReplaceText(&ui.TextInputRange{Start: 0, End: 4}, "text")
	tt.Frame()
	if calls != 0 || d.Text().String() != "textword" {
		t.Fatalf("non-typing called hook: text %q calls %d", d.Text().String(), calls)
	}
	opts.ReadOnly = true
	tt.Frame()
	tt.Type("(")
	tt.Key(0, ui.KeyBackspace)
	(*state).SetMarkedText(nil, "（", ui.TextInputRange{Start: 1, End: 1})
	(*state).ReplaceText(nil, "（")
	if calls != 0 || d.Text().String() != "textword" {
		t.Fatalf("read-only input changed text %q calls %d", d.Text().String(), calls)
	}
}

func TestTextInputHookCompositionAtCaret(t *testing.T) {
	for _, finish := range []string{"direct", "end-preedit-replace", "cancel"} {
		t.Run(finish, func(t *testing.T) {
			d := NewDocument(NewText("a"))
			calls := 0
			opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool { calls++; return hookPair(e, s) }}
			tt, _, state := inputHookTester(t, d, &opts)
			d.SetSelection(1, 1)
			tt.Frame()
			tt.Compose("(", 1)
			switch finish {
			case "direct":
				tt.Type("(")
			case "end-preedit-replace":
				(*state).EndPreedit()
				if calls != 0 {
					t.Fatal("hook saw provisional text")
				}
				(*state).ReplaceText(&ui.TextInputRange{Start: 1, End: 2}, "（")
				tt.Frame()
			case "cancel":
				tt.Compose("", 0)
				if calls != 0 || d.Text().String() != "a" {
					t.Fatalf("cancel called hook %d or changed text %q", calls, d.Text().String())
				}
				return
			}
			want := "a()"
			if finish == "end-preedit-replace" {
				want = "a（）"
			}
			if calls != 1 || d.Text().String() != want {
				t.Fatalf("commit calls %d text %q", calls, d.Text().String())
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if got := d.Text().String(); got != "a" {
				t.Fatalf("composition split undo: %q", got)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if got := d.Text().String(); got != want {
				t.Fatalf("composition redo %q", got)
			}
		})
	}
}

func TestTextInputHookTypingAttributesAndDisabled(t *testing.T) {
	d := NewDocument(NewText(""))
	opts := EditorOptions{OnTextInput: hookPair}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetTypingAttributes(Attributes{Italic: true})
	tt.Frame()
	tt.Type("(")
	if !d.Text().AttributesAt(0).Italic || !d.Text().AttributesAt(1).Italic {
		t.Fatal("hook lost the explicit typing attributes")
	}
	opts.OnTextInput = nil
	tt.Frame()
	tt.Type("(")
	if got := d.Text().String(); got != "(()" {
		t.Fatalf("disabled hook %q", got)
	}
}

func TestTextInputHookRawInputAndMarkdown(t *testing.T) {
	d := NewDocument(NewText(""))
	opts := EditorOptions{OnTextInput: hookPair}
	tt, _, state := inputHookTester(t, d, &opts)
	(*state).input(ui.InputEvent{Kind: ui.InputText, Text: "("})
	tt.Frame()
	if d.Text().String() != "()" {
		t.Fatalf("raw input %q", d.Text().String())
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	typeRunes(tt, "**bold**")
	if got := d.Text().String(); got != "bold" || !d.Text().AttributesAt(0).Bold() {
		t.Fatalf("declined hook broke Markdown shortcuts: %q", got)
	}
}

func TestBackspaceInputHookDuringComposition(t *testing.T) {
	d := NewDocument(NewText("()"))
	calls := 0
	opts := EditorOptions{OnBackspace: func(*Edit) bool { calls++; return false }}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetSelection(1, 1)
	tt.Frame()
	tt.Compose("字", 1)
	(*state).input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace})
	if calls != 0 {
		t.Fatal("backspace hook ran during composition")
	}
}

func TestTextInputHookHandledMarkerRunsMarkdownShortcut(t *testing.T) {
	d := NewDocument(NewText("*word*"))
	opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
		if s != "*" {
			return false
		}
		e.Select(6, 6)
		return true
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(5, 5)
	tt.Frame()
	tt.Type("*")
	if got := d.Text().String(); got != "word" || !d.Text().AttributesAt(0).Italic {
		t.Fatalf("handled marker did not convert: %q", got)
	}
}

func TestTextInputHookComposeAcrossParagraphs(t *testing.T) {
	original := ParseMarkdown("# Head\n\none **bold**\n\nlast")
	for _, handled := range []bool{false, true} {
		t.Run(map[bool]string{false: "decline", true: "wrap"}[handled], func(t *testing.T) {
			d := NewDocument(original)
			opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
				if got := e.Text().Markdown(); got != original.Markdown() {
					t.Fatalf("composition restoration changed paragraph styles: %q", got)
				}
				return handled && hookPair(e, s)
			}}
			tt, _, _ := inputHookTester(t, d, &opts)
			d.SetSelection(2, 12)
			tt.Frame()
			tt.Compose("(", 1)
			tt.Type("(")
			if !handled {
				native := NewDocument(original)
				native.SetSelection(2, 12)
				native.InsertText("(")
				if got := d.Text().Markdown(); got != native.Text().Markdown() {
					t.Fatalf("declined hook %q differs from native %q", got, native.Text().Markdown())
				}
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if got := d.Text().Markdown(); got != original.Markdown() {
				t.Fatalf("cross-paragraph undo %q", got)
			}
		})
	}
}

func TestTextInputHookMarkedRangeUpdateKeepsOriginal(t *testing.T) {
	d := NewDocument(NewText("word"))
	calls := 0
	opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
		calls++
		if e.Text().String() != "word" || !e.TypingAttributes().Italic {
			t.Fatalf("lost original selection or typing attributes: %q %+v", e.Text().String(), e.TypingAttributes())
		}
		return false
	}}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetSelection(0, 4)
	d.SetTypingAttributes(Attributes{Italic: true})
	tt.Frame()
	tt.Compose("ab", 2)
	marked, _ := (*state).MarkedRange()
	(*state).SetMarkedText(&marked, "（", ui.TextInputRange{Start: 1, End: 1})
	if calls != 0 {
		t.Fatal("marked range update called the hook")
	}
	tt.Type("（")
	if calls != 1 || d.Text().String() != "（" || !d.Text().AttributesAt(0).Italic {
		t.Fatalf("commit lost explicit input format: %q", d.Text().Markdown())
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if d.Text().String() != "word" {
		t.Fatalf("undo %q", d.Text().String())
	}
}

func TestTextInputHookRestoresParagraphBoundary(t *testing.T) {
	original := ParseMarkdown("# Head\n\nbody")
	d := NewDocument(original)
	opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
		if e.Text().Markdown() != original.Markdown() {
			t.Fatalf("restoration changed the tail paragraph style: %q", e.Text().Markdown())
		}
		return hookPair(e, s)
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(2, 5)
	tt.Frame()
	tt.Compose("(", 1)
	tt.Type("(")
	if got := d.Text().Markdown(); got != "# He(ad\n\n)body" {
		t.Fatalf("wrapped styles %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if d.Text().Markdown() != original.Markdown() {
		t.Fatalf("undo %q", d.Text().Markdown())
	}
}

func TestBackspaceInputHookAfterFinalUnmark(t *testing.T) {
	d := NewDocument(NewText(""))
	opts := EditorOptions{OnTextInput: hookPair, OnBackspace: func(e *Edit) bool {
		if e.Text().String() != "()" {
			return false
		}
		e.ReplaceString(0, 2, "", Attributes{})
		e.Select(0, 0)
		return true
	}}
	tt, _, state := inputHookTester(t, d, &opts)
	tt.Compose("(", 1)
	(*state).UnmarkText()
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	if d.Text().String() != "" {
		t.Fatalf("backspace after unmark %q", d.Text().String())
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if d.Text().String() != "()" {
		t.Fatalf("undo %q", d.Text().String())
	}
}

func TestTextInputHookSelectionOnlyResetsNavigation(t *testing.T) {
	d := NewDocument(NewText("()"))
	opts := EditorOptions{OnTextInput: hookPair}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetSelection(1, 1)
	tt.Frame()
	(*state).hasDesired, (*state).affinity = true, ui.TextUpstream
	tt.Type(")")
	if (*state).hasDesired || (*state).affinity != ui.TextDownstream {
		t.Fatal("selection-only hook retained vertical navigation state")
	}
}

func TestTextInputHookUnmarkAfterExternalSelection(t *testing.T) {
	d := NewDocument(NewText("word"))
	calls := 0
	opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool { calls++; return hookPair(e, s) }}
	tt, _, state := inputHookTester(t, d, &opts)
	d.SetSelection(1, 3)
	tt.Frame()
	tt.Compose("(", 1)
	d.SetSelection(0, 0)
	(*state).UnmarkText()
	if calls != 0 || d.Text().String() != "wd" {
		t.Fatalf("stale composition restored text: %q calls %d", d.Text().String(), calls)
	}
}

func TestEditSetTypingAttributes(t *testing.T) {
	d := NewDocument(NewText("seed"))
	d.SetSelection(4, 4)
	before, version := d.Text().Markdown(), d.Version()
	attrs := Attributes{Font: "serif", Size: 18, Weight: 700, Italic: true,
		Underline: true, Strikethrough: true, Code: true, Link: "https://example.test",
		LinkTitle: "title", Tag: "tag", Attachment: &Attachment{}, SoftBreak: true, SoftBreakBefore: true}
	d.SetTypingAttributes(attrs)
	want := d.TypingAttributes()
	d.SetSelection(4, 4)
	d.Edit(func(e *Edit) {
		e.SetTypingAttributes(attrs)
		if got := e.TypingAttributes(); got != want || got.Attachment != nil || got.SoftBreak || got.SoftBreakBefore {
			t.Fatalf("typing attributes = %+v, want %+v", got, want)
		}
	})
	if d.Text().Markdown() != before || d.Version() != version || d.CanUndo() {
		t.Fatal("setting typing attributes changed the text, version or undo history")
	}
	if a, c := d.Selection(); a != 4 || c != 4 {
		t.Fatalf("selection = %d, %d", a, c)
	}
	d.Edit(func(e *Edit) {
		e.Select(0, 0)
		if e.TypingAttributes() == want {
			t.Fatal("moving the selection retained explicit typing attributes")
		}
	})
}

func TestBackspaceInputHookRestoresTypingAttributes(t *testing.T) {
	for _, sameFrame := range []bool{false, true} {
		name := "separate frames"
		if sameFrame {
			name = "same frame"
		}
		t.Run(name, func(t *testing.T) {
			d := NewDocument(NewText("seed()"))
			opts := EditorOptions{OnBackspace: func(e *Edit) bool {
				attrs := e.TypingAttributes()
				e.ReplaceString(4, 6, "", Attributes{})
				e.Select(4, 4)
				e.SetTypingAttributes(attrs)
				return true
			}}
			tt, _, _ := inputHookTester(t, d, &opts)
			d.SetSelection(5, 5)
			tt.Frame()
			d.SetTypingAttributes(Attributes{Weight: 700, Italic: true})
			action := func() {
				tt.Key(0, ui.KeyBackspace)
				tt.Type("x")
			}
			if sameFrame {
				tt.Batch(action)
			} else {
				action()
			}
			if got := d.Text().Markdown(); got != "seed***x***" {
				t.Fatalf("typed markdown = %q", got)
			}
			if a, c := d.Selection(); a != 5 || c != 5 {
				t.Fatalf("typed selection = %d, %d", a, c)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if got := d.Text().String(); got != "seed" {
				t.Fatalf("undo typing = %q", got)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if got := d.Text().Markdown(); got != "seed()" {
				t.Fatalf("undo deletion = %q", got)
			}
			if a, c := d.Selection(); a != 5 || c != 5 {
				t.Fatalf("undo selection = %d, %d", a, c)
			}
			if d.CanUndo() {
				t.Fatal("restoring typing attributes added an undo step")
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if got := d.Text().Markdown(); got != "seed***x***" {
				t.Fatalf("redo typing = %q", got)
			}
		})
	}
}
