package richtext

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func accessibleAttachmentSource() Text {
	return NewText("🙂a").
		Append(string(ObjectReplacement), Attributes{Weight: 700, Attachment: &Attachment{
			Name: "copy name", AccessibleText: "漢[かん]", Width: 30, Height: 20,
		}}).
		Append("β\n", Attributes{Italic: true}).
		Append(string(ObjectReplacement), Attributes{Attachment: &Attachment{Name: "legacy", Width: 20, Height: 20}}).
		Append("z", Attributes{}).
		Append(string(ObjectReplacement), Attributes{Attachment: &Attachment{AccessibleText: "字🙂", Width: 30, Height: 20}}).
		Append("!", Attributes{})
}

func TestAttachmentAccessibleValueAndRanges(t *testing.T) {
	original := accessibleAttachmentSource()
	d := NewDocument(original)
	opts := EditorOptions{}
	tt, changed, state := inputHookTester(t, d, &opts)
	version := d.Version()
	want := "🙂a漢[かん]β\n￼z字🙂!"
	if got, ok := tt.Access("Body", ui.RoleTextField); !ok || got.Value != want {
		t.Fatalf("accessible value %+v, found=%v", got, ok)
	}
	for source, expanded := range []int{0, 1, 2, 7, 8, 9, 10, 11, 13, 14} {
		d.SetSelection(source, source)
		tt.Frame()
		var n platform.AccessNode
		(*state).access(&n)
		if n.SelStart != expanded || n.SelEnd != expanded {
			t.Fatalf("source %d: accessible selection %d,%d, want %d", source, n.SelStart, n.SelEnd, expanded)
		}
	}
	for _, reverse := range []bool{false, true} {
		a, z := 2, 8
		if reverse {
			a, z = z, a
		}
		d.SetSelection(a, z)
		tt.Frame()
		var n platform.AccessNode
		(*state).access(&n)
		if n.SelStart != 2 || n.SelEnd != 13 {
			t.Fatalf("expanded selection %d,%d", n.SelStart, n.SelEnd)
		}
		// Native accessibility backends query Value, using UTF-16 on macOS
		// and rune ranges on Linux and Windows. Neither queries IME text.
		a16, z16 := ui.UTF16Len("🙂a"), ui.UTF16Len("🙂a漢[かん]β\n￼z字🙂")
		selected := n.Value[ui.UTF16ByteOffset(n.Value, a16):ui.UTF16ByteOffset(n.Value, z16)]
		if selected != "漢[かん]β\n￼z字🙂" {
			t.Fatalf("native UTF-16 range %q", selected)
		}
		bold, italic := false, false
		for _, r := range n.TextRuns {
			if r.Weight == 700 && (r.Start != 2 || r.End != 7) || r.Italic && (r.Start != 7 || r.End != 8) {
				t.Fatalf("accessible format range %+v", r)
			}
			bold = bold || r.Weight == 700
			italic = italic || r.Italic
		}
		if !bold || !italic {
			t.Fatal("accessible formats were lost")
		}
	}
	if !d.Text().Same(original) || d.Version() != version || d.CanUndo() || *changed {
		t.Fatal("reading accessible text or selecting changed document/history")
	}
}

func TestAttachmentAccessibleSelectionEditing(t *testing.T) {
	for _, tc := range []struct {
		name          string
		a, z          int
		anchor, caret int
	}{
		{"interior", 3, 4, 2, 3}, {"reverse", 4, 3, 3, 2},
		{"caret", 5, 5, 2, 2}, {"exact", 2, 7, 2, 3},
		{"after", 7, 8, 3, 4}, {"second", 12, 13, 7, 8},
		{"multiple", 4, 12, 2, 8}, {"reverse multiple", 12, 4, 8, 2},
		{"past end", 1000, 1000, 9, 9}, {"before start", -10, -10, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := accessibleAttachmentSource()
			d := NewDocument(original)
			opts := EditorOptions{NoMarkdown: true}
			tt, _, state := inputHookTester(t, d, &opts)
			if !(*state).input(ui.InputEvent{Kind: ui.InputSelect, Start: tc.a, End: tc.z}) {
				t.Fatal("accessible selection was not handled")
			}
			if a, z := d.Selection(); a != tc.anchor || z != tc.caret {
				t.Fatalf("source selection %d,%d, want %d,%d", a, z, tc.anchor, tc.caret)
			}
			// Type before a frame to verify the mapped selection is published.
			tt.Type("X")
			a, z := min(tc.anchor, tc.caret), max(tc.anchor, tc.caret)
			want := original.Slice(0, a).String() + "X" + original.Slice(z, original.Len()).String()
			if got := d.Text().String(); got != want {
				t.Fatalf("edited %q, want %q", got, want)
			}
			d.Undo()
			tt.Frame()
			if !d.Text().Equal(original) || d.CanUndo() {
				t.Fatal("accessible edit did not undo in one step")
			}
		})
	}
}

func TestAttachmentAccessibleInputAndClipboardStayOriginal(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		original := accessibleAttachmentSource()
		d := NewDocument(original)
		opts := EditorOptions{ReadOnly: readOnly}
		tt, _, state := inputHookTester(t, d, &opts)
		(*state).input(ui.InputEvent{Kind: ui.InputSelect, Start: 3, End: 4})
		tt.Key(ui.Cmd, ui.KeyC)
		selected := original.Slice(2, 3)
		if got := tt.Clipboard(); got != selected.PlainText() {
			t.Fatalf("clipboard %q, want original selection", got)
		}
		data := tt.ClipboardData()
		if b, err := data.Read(transfer.HTML); err != nil || string(b) != selected.HTML() {
			t.Fatalf("HTML clipboard %q, %v", b, err)
		}
		if b, err := data.Read(Format); err != nil {
			t.Fatal(err)
		} else if got, ok := decodeClip(b); !ok || !got.Equal(selected) {
			t.Fatal("rich clipboard changed the original object")
		}
		if !d.Text().Same(original) || d.CanUndo() {
			t.Fatal("accessible selection/copy changed source")
		}
		// Input methods retain UTF-16 coordinates of the one-rune object.
		v := *state
		text, actual := v.TextForRange(ui.TextInputRange{Start: 3, End: 4})
		if text != string(ObjectReplacement) || actual.Start != 3 || actual.End != 4 {
			t.Fatalf("IME query %q at %+v", text, actual)
		}
		if s := v.Selection(); s.Range.Start != 3 || s.Range.End != 4 {
			t.Fatalf("IME selection %+v", s)
		}
		v.ReplaceText(&ui.TextInputRange{Start: 3, End: 4}, "Q")
		tt.Frame()
		if readOnly {
			if !d.Text().Same(original) {
				t.Fatal("read-only native edit changed source")
			}
		} else {
			if d.Text().String() != "🙂aQβ\n￼z￼!" {
				t.Fatalf("native edit %q", d.Text().String())
			}
			d.Undo()
			tt.Frame()
			if !d.Text().Equal(original) || d.CanUndo() {
				t.Fatal("native edit did not undo once")
			}
		}
	}
}

func TestAttachmentAccessibleFallbackAndLongText(t *testing.T) {
	for _, source := range []Text{NewText(""), NewText("plain 🙂\ntext"), NewText(string(ObjectReplacement)),
		TextOf(Paragraph{Runs: []Run{{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Name: "legacy"}}}}}),
	} {
		d := NewDocument(source)
		tt, _, _ := inputHookTester(t, d, &EditorOptions{})
		if n, ok := tt.Access("Body", ui.RoleTextField); !ok || n.Value != source.String() {
			t.Fatalf("fallback %+v, found=%v", n, ok)
		}
	}
	const count = 512
	alt := strings.Repeat("字🙂", 128)
	source := TextOf(Paragraph{Runs: []Run{{Text: strings.Repeat(string(ObjectReplacement), count),
		Attributes: Attributes{Attachment: &Attachment{AccessibleText: alt}}}}})
	m := source.accessible()
	if m.value != strings.Repeat(alt, count) || m.length != count*256 || len(m.objects) != count {
		t.Fatal("long accessible attachment text was truncated or miscounted")
	}
	for i := 0; i <= count; i++ {
		if got := m.offset(i); got != i*256 {
			t.Fatalf("long source %d maps to %d", i, got)
		}
	}
}
