package richtext

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// cropTester builds an editor 500 wide of "ab", an attachment w by h,
// cropped, and "cd", whose drawing records its box and whose scrolls
// OnAttachmentScroll takes while take is set.
func cropTester(t *testing.T, w, h float32, take *bool) (*ui.Tester, *Scroll, *ui.Rect, *[]ui.InputEvent, *[][2]float32) {
	t.Helper()
	drawn := &ui.Rect{}
	at := &Attachment{Name: "wide", Width: w, Height: h, Crop: true, Draw: func(_ *ui.Painter, r ui.Rect) { *drawn = r }}
	doc := NewDocument(TextOf(
		Paragraph{Runs: []Run{{Text: "ab"}}},
		Paragraph{Runs: []Run{{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: at}}}},
		Paragraph{Runs: []Run{{Text: "cd " + longLines(80)}}},
	))
	doc.SetSelection(0, 0)
	scroll := &Scroll{}
	var seen []ui.InputEvent
	var where [][2]float32
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{
			Scroll: scroll,
			OnAttachmentScroll: func(i int, x, y float32, ev ui.InputEvent) bool {
				if i != 3 {
					t.Errorf("scroll over rune %d", i)
				}
				seen = append(seen, ev)
				where = append(where, [2]float32{x, y})
				return *take
			},
		}).Fill()
	}, 500, 300)
	tt.Frame()
	return tt, scroll, drawn, &seen, &where
}

func longLines(n int) string {
	s := ""
	for range n {
		s += "a line of text to scroll by. "
	}
	return s
}

// A cropped box wider than the column keeps its height, cut to the
// column, and is drawn in that box.
func TestAttachmentCrop(t *testing.T) {
	take := false
	_, scroll, drawn, _, _ := cropTester(t, 2000, 120, &take)
	box, ok := scroll.AttachmentBox(3)
	_, col := scroll.Column()
	if !ok || box.W != col || box.H != 120 {
		t.Fatalf("box %+v, column %v", box, col)
	}
	if drawn.W != box.W || drawn.H != box.H {
		t.Fatalf("drawn in %+v, box %+v", *drawn, box)
	}
}

// A narrow cropped box is as wide as it says.
func TestAttachmentCropNarrow(t *testing.T) {
	take := false
	_, scroll, _, _, _ := cropTester(t, 200, 80, &take)
	if box, ok := scroll.AttachmentBox(3); !ok || box.W != 200 || box.H != 80 {
		t.Fatalf("box %+v", box)
	}
}

// A press on a cropped box is at the point in the part that shows,
// unscaled.
func TestAttachmentCropPress(t *testing.T) {
	at := &Attachment{Width: 2000, Height: 100, Crop: true, Draw: func(*ui.Painter, ui.Rect) {}}
	doc := NewDocument(TextOf(Paragraph{Runs: []Run{{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: at}}}}))
	scroll := &Scroll{}
	var got [2]float32
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{Scroll: scroll, OnAttachment: func(i int, x, y float32, ev ui.InputEvent) bool {
			got = [2]float32{x, y}
			return true
		}}).Fill()
	}, 500, 300)
	tt.Frame()
	box, _ := scroll.AttachmentBox(0)
	tt.ClickAt(box.X+300, box.Y+40)
	if got[0] < 299 || got[0] > 301 || got[1] < 39 || got[1] > 41 {
		t.Fatalf("pressed at %v, want 300,40", got)
	}
}

// A scroll over an attachment goes to the app first; one the app leaves
// scrolls the editor, as one beside it does.
func TestAttachmentScroll(t *testing.T) {
	take := true
	tt, scroll, _, seen, where := cropTester(t, 2000, 120, &take)
	box, _ := scroll.AttachmentBox(3)
	tt.Scroll(box.X+50, box.Y+30, 40, 0)
	if len(*seen) != 1 || (*seen)[0].DX != 40 || (*seen)[0].DY != 0 {
		t.Fatalf("scrolls %+v", *seen)
	}
	if w := (*where)[0]; w[0] < 49 || w[0] > 51 || w[1] < 29 || w[1] > 31 {
		t.Fatalf("scrolled at %v, want 50,30", w)
	}
	if scroll.Offset() != 0 {
		t.Fatal("a scroll taken scrolled the editor")
	}

	take = false
	tt.Scroll(box.X+50, box.Y+30, 0, 30)
	if len(*seen) != 2 || scroll.Offset() != 30 {
		t.Fatalf("scrolls %d, offset %v", len(*seen), scroll.Offset())
	}

	// Below the box, the app is not asked.
	tt.Scroll(box.X+50, box.Y+box.H+60, 0, 10)
	if len(*seen) != 2 {
		t.Fatalf("a scroll off the attachment reached it: %+v", *seen)
	}
}

// The box of a block of code is the run of paragraphs of code around the
// rune, and its label at the top right.
func TestCodeBlockBox(t *testing.T) {
	code := ParagraphStyle{CodeBlock: true, Language: "go"}
	doc := NewDocument(TextOf(
		Paragraph{Runs: []Run{{Text: "before"}}},
		Paragraph{Style: code, Runs: []Run{{Text: "a := 1"}}},
		Paragraph{Style: code, Runs: []Run{{Text: "b := 2"}}},
		Paragraph{Style: code, Runs: []Run{{Text: "c := 3"}}},
		Paragraph{Runs: []Run{{Text: "after"}}},
	))
	scroll := &Scroll{}
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{Scroll: scroll, Theme: &Theme{CodeLanguageLabel: true}}).Fill()
	}, 500, 400)
	tt.Frame()
	if _, _, ok := scroll.CodeBlockBox(2); ok {
		t.Fatal("a paragraph of text has a block of code")
	}
	start := len("before\n")
	box, label, ok := scroll.CodeBlockBox(start + 2)
	if !ok {
		t.Fatal("no block of code")
	}
	for _, i := range []int{start, start + 7, start + 14 + 6} {
		b, l, ok := scroll.CodeBlockBox(i)
		if !ok || b != box || l != label {
			t.Fatalf("rune %d: %+v %+v, want %+v %+v", i, b, l, box, label)
		}
	}
	x, w := scroll.Column()
	if box.X != x || box.W != w || box.H <= 0 {
		t.Fatalf("box %+v, column %v %v", box, x, w)
	}
	// The lines are inside the box, the text before and after outside.
	for i, in := range map[int]bool{0: false, start: true, start + 20: true, start + 21: false} {
		_, cy, ch := scroll.v.caretAt(i, 0, ui.TextDownstream)
		y := float32(cy) - scroll.Offset() + scroll.v.padY
		if inside := y >= box.Y && y+ch <= box.Y+box.H; inside != in {
			t.Errorf("rune %d at %v-%v, box %+v", i, y, y+ch, box)
		}
	}
	if label.W <= 0 || label.X+label.W > box.X+box.W || label.X < box.X+box.W/2 || label.Y < box.Y {
		t.Fatalf("label %+v in %+v", label, box)
	}
	first := scroll.ParaTop(start)
	if label.Y+label.H > first {
		t.Fatalf("label %+v over the first line at %v", label, first)
	}
	// Without a language, no label.
	doc.Edit(func(e *Edit) {
		e.FormatParagraphs(start, start, func(s *ParagraphStyle) { s.Language = "" })
	})
	tt.Frame()
	if _, l, ok := scroll.CodeBlockBox(start); !ok || l != (ui.Rect{}) {
		t.Fatalf("label %+v", l)
	}
}
