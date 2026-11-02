package richtext

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// atomicInlineInput models an app wrapping a selection or crossing an
// existing closing mark, with conversion belonging to that input's edit.
func atomicInlineInput(e *Edit, s string) bool {
	if !strings.Contains("*_`~", s) || utf8.RuneCountInString(s) != 1 {
		return false
	}
	a, c := e.Selection()
	start, end := min(a, c), max(a, c)
	mark := s
	if s == "~" {
		mark = "~~"
	}
	n := utf8.RuneCountInString(mark)
	if start != end {
		e.ReplaceString(end, end, mark, Attributes{})
		e.ReplaceString(start, start, mark, Attributes{})
		e.Select(a+n, c+n)
		e.ConvertInlineMarkdown(end + 2*n)
		return true
	}
	if c < e.Text().Len() && e.Text().Slice(c, c+1).String() == s {
		e.Select(c+1, c+1)
		e.ConvertInlineMarkdown(c + 1)
		return true
	}
	e.ReplaceString(c, c, s, e.TypingAttributes())
	e.Select(c+1, c+1)
	e.ConvertInlineMarkdown(c + 1)
	return true
}

func TestInlineEditClosingAtomicHistory(t *testing.T) {
	for _, syntax := range []string{"*dad*", "_dad_", "**why?**", "__why?__", "`dad`", "~~dad~~"} {
		for _, native := range []bool{false, true} {
			t.Run(syntax+map[bool]string{false: "/text", true: "/native"}[native], func(t *testing.T) {
				d := NewDocument(NewText(syntax))
				opts := EditorOptions{OnTextInput: atomicInlineInput}
				tt, _, state := inputHookTester(t, d, &opts)
				n := utf8.RuneCountInString(syntax)
				d.SetSelection(n-1, n-1)
				tt.Frame()
				if native {
					(*state).ReplaceText(nil, syntax[len(syntax)-1:])
					tt.Frame()
				} else {
					tt.Type(syntax[len(syntax)-1:])
				}
				want := strings.Trim(syntax, "*_`~")
				if got := d.Text().String(); got != want {
					t.Fatalf("converted %q, want %q", got, want)
				}
				attrs := d.Text().AttributesAt(0)
				switch syntax[0] {
				case '*', '_':
					if strings.HasPrefix(syntax, "**") || strings.HasPrefix(syntax, "__") {
						if !attrs.Bold() {
							t.Fatal("missing bold")
						}
					} else if !attrs.Italic {
						t.Fatal("missing italic")
					}
				case '`':
					if !attrs.Code {
						t.Fatal("missing code")
					}
				case '~':
					if !attrs.Strikethrough {
						t.Fatal("missing strike")
					}
				}
				if a, c := d.Selection(); a != len(want) || c != len(want) {
					t.Fatalf("converted selection %d,%d", a, c)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				if d.Text().String() != syntax {
					t.Fatalf("undo %q", d.Text().String())
				}
				if a, c := d.Selection(); a != n-1 || c != n-1 {
					t.Fatalf("undo selection %d,%d", a, c)
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if d.Text().String() != want {
					t.Fatalf("redo %q", d.Text().String())
				}
				tt.Type("!")
				if a := d.Text().AttributesAt(len(want)); a.Italic || a.Bold() || a.Code || a.Strikethrough {
					t.Fatal("format leaked after closing")
				}
			})
		}
	}
}

func TestInlineEditWrapSelection(t *testing.T) {
	for _, mark := range []string{"*", "_", "`", "~"} {
		for _, reverse := range []bool{false, true} {
			t.Run(mark+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				original := ParseMarkdown("# Keep\n\nA **选区** tail\n\nlast")
				d := NewDocument(original)
				opts := EditorOptions{OnTextInput: atomicInlineInput}
				tt, _, _ := inputHookTester(t, d, &opts)
				a, c := 7, 9
				if reverse {
					a, c = c, a
				}
				d.SetSelection(a, c)
				tt.Frame()
				tt.Type(mark)
				if d.Text().String() != original.String() {
					t.Fatalf("wrap changed text %q", d.Text().String())
				}
				if x, y := d.Selection(); x != a || y != c {
					t.Fatalf("wrapped selection %d,%d want %d,%d", x, y, a, c)
				}
				if !d.Text().AttributesAt(7).Bold() {
					t.Fatal("existing bold lost")
				}
				if d.Text().Markdown() == original.Markdown() {
					t.Fatal("format was not applied")
				}
				formatted := d.Text()
				tt.Key(ui.Cmd, ui.KeyZ)
				if !d.Text().Same(original) {
					t.Fatal("undo did not restore original attributed text")
				}
				if x, y := d.Selection(); x != a || y != c {
					t.Fatal("undo selection changed")
				}
				tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
				if !d.Text().Same(formatted) {
					t.Fatal("redo did not restore formatted text")
				}
				if x, y := d.Selection(); x != a || y != c {
					t.Fatal("redo selection changed")
				}
			})
		}
	}
}

func TestInlineEditSameFrameAndComposition(t *testing.T) {
	for _, commit := range []string{"batch", "type", "unmark", "range", "end-preedit"} {
		t.Run(commit, func(t *testing.T) {
			d := NewDocument(NewText("seed"))
			calls := 0
			opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool { calls++; return atomicInlineInput(e, s) }}
			tt, _, state := inputHookTester(t, d, &opts)
			d.SetSelection(4, 0)
			tt.Frame()
			if commit == "batch" {
				tt.Batch(func() { tt.Type("*"); tt.Type("x") })
			} else {
				tt.Compose("star", 4)
				tt.Compose("*", 1)
				if calls != 0 {
					t.Fatal("hook saw provisional composition")
				}
				switch commit {
				case "type":
					tt.Type("*")
				case "unmark":
					(*state).UnmarkText()
					tt.Frame()
				case "range":
					(*state).ReplaceText(&ui.TextInputRange{Start: 0, End: 1}, "*")
					tt.Frame()
				case "end-preedit":
					(*state).EndPreedit()
					(*state).ReplaceText(&ui.TextInputRange{Start: 0, End: 1}, "*")
					tt.Frame()
				}
			}
			if commit == "batch" {
				if got := d.Text().String(); got != "x" {
					t.Fatalf("same-frame replacement %q", got)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
			}
			if got := d.Text().String(); got != "seed" || !d.Text().AttributesAt(0).Italic || calls != map[bool]int{false: 1, true: 2}[commit == "batch"] {
				t.Fatalf("commit text %q calls %d", got, calls)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if d.Text().String() != "seed" || d.Text().AttributesAt(0).Italic {
				t.Fatal("undo composition did not restore original")
			}
			if a, c := d.Selection(); a != 4 || c != 0 {
				t.Fatalf("undo composition selection %d,%d", a, c)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if !d.Text().AttributesAt(0).Italic {
				t.Fatal("redo composition lost formatting")
			}
		})
	}
}

func TestInlineEditNoMatchAndCodeBlock(t *testing.T) {
	for _, source := range []string{"plain", "*open", "snake_case_", "~one~", "* padded*", "````"} {
		d := NewDocument(NewText(source))
		d.SetSelection(0, 0)
		d.Edit(func(e *Edit) {
			if e.ConvertInlineMarkdown(e.Text().Len()) {
				t.Fatalf("unexpected conversion %q", source)
			}
		})
		if d.CanUndo() || d.Text().String() != source {
			t.Fatalf("no match changed %q", source)
		}
	}
	d := NewDocument(ParseMarkdown("```\n*word*\n```"))
	original := d.Text()
	d.Edit(func(e *Edit) {
		if e.ConvertInlineMarkdown(6) {
			t.Fatal("converted code block")
		}
	})
	if !d.Text().Same(original) || d.CanUndo() {
		t.Fatal("code block changed")
	}
}

func TestInlineEditConsumptionIsPerHandledInput(t *testing.T) {
	for _, handled := range []bool{false, true} {
		t.Run(map[bool]string{false: "declined", true: "handled"}[handled], func(t *testing.T) {
			d := NewDocument(NewText("*dad"))
			calls := 0
			opts := EditorOptions{OnTextInput: func(e *Edit, s string) bool {
				calls++
				if calls > 1 {
					return false
				}
				if e.ConvertInlineMarkdown(0) {
					t.Fatal("unexpected conversion")
				}
				if handled {
					a, c := e.Selection()
					e.ReplaceString(a, c, s, e.TypingAttributes())
					e.Select(c+1, c+1)
				}
				return handled
			}}
			tt, _, _ := inputHookTester(t, d, &opts)
			tt.Type("*")
			if handled {
				if d.Text().String() != "*dad*" {
					t.Fatalf("failed conversion did not consume default: %q", d.Text().String())
				}
			} else if d.Text().String() != "dad" || !d.Text().AttributesAt(0).Italic {
				t.Fatal("declined hook suppressed default conversion")
			}
			d.SetText(NewText("*dad"))
			tt.Frame()
			tt.Type("*")
			if d.Text().String() != "dad" || !d.Text().AttributesAt(0).Italic {
				t.Fatal("consumption leaked into next input")
			}
		})
	}
}

func TestInlineEditRestoresExplicitTyping(t *testing.T) {
	d := NewDocument(NewText("*dad*"))
	d.SetSelection(4, 4)
	original := Attributes{Underline: true, Weight: 700}
	d.SetTypingAttributes(original)
	d.Edit(func(e *Edit) {
		e.Select(5, 5)
		if !e.ConvertInlineMarkdown(5) {
			t.Fatal("did not convert")
		}
	})
	converted := d.TypingAttributes()
	if converted.Italic || converted.Bold() || converted.Underline {
		t.Fatal("typing did not leave format")
	}
	d.Undo()
	if got := d.TypingAttributes(); got != original {
		t.Fatalf("undo typing %+v, want %+v", got, original)
	}
	d.Redo()
	if got := d.TypingAttributes(); got != converted {
		t.Fatalf("redo typing %+v, want %+v", got, converted)
	}
}

func TestInlineEditPublishesMappedSelection(t *testing.T) {
	for _, sel := range [][2]int{{2, 2}, {2, 3}, {3, 2}} {
		for _, frame := range []bool{false, true} {
			name := fmt.Sprintf("%d-%d/frame=%t", sel[0], sel[1], frame)
			t.Run(name, func(t *testing.T) {
				d := NewDocument(NewText("*dad*"))
				opts := EditorOptions{}
				tt, _, state := inputHookTester(t, d, &opts)
				d.SetSelection(sel[0], sel[1])
				tt.Frame()
				d.Edit(func(e *Edit) {
					if !e.ConvertInlineMarkdown(5) {
						t.Fatal("conversion failed")
					}
				})
				if a, c := d.Selection(); a != sel[0]-1 || c != sel[1]-1 {
					t.Fatalf("document selection %d,%d", a, c)
				}
				if frame {
					tt.Frame()
					if v := *state; v.selAnchor != sel[0]-1 || v.caret != sel[1]-1 {
						t.Fatalf("view selection %d,%d", v.selAnchor, v.caret)
					}
				}
				want := "dxd"
				if sel[0] == sel[1] {
					want = "dxad"
				}
				tt.Type("x")
				if got := d.Text().String(); got != want {
					t.Fatalf("immediate input %q, want %q", got, want)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				tt.Key(ui.Cmd, ui.KeyZ)
				if d.Text().String() != "*dad*" {
					t.Fatal("conversion undo text changed")
				}
				if a, c := d.Selection(); a != sel[0] || c != sel[1] {
					t.Fatalf("undo selection %d,%d", a, c)
				}
				d.Redo()
				tt.Frame()
				if a, c := d.Selection(); a != sel[0]-1 || c != sel[1]-1 {
					t.Fatalf("redo selection %d,%d", a, c)
				}
				tt.Type("x")
				if got := d.Text().String(); got != want {
					t.Fatalf("typed %q, want %q", got, want)
				}
				if d.Text().AttributesAt(1).Italic {
					t.Fatal("typing inherited converted format")
				}
			})
		}
	}
}
