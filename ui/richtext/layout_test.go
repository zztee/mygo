package richtext

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// baseTester builds an EditorBase of doc with opts filling a 500×400
// window.
func baseTester(doc *Document, opts EditorOptions) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, opts).Fill()
	}, 500, 400)
	tt.Frame()
	tt.Frame()
	return tt
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }

// A MaxWidth column is centered, and its sides take the pointer.
func TestRichTextColumn(t *testing.T) {
	doc := NewDocument(NewText("hello world"))
	var s Scroll
	tt := baseTester(doc, EditorOptions{MaxWidth: 200, Margin: [4]float32{0, 10, 0, 30}, Scroll: &s})
	x, w := s.Column()
	if !near(w, 200) || !near(x, 30+(500-40-200)/2) {
		t.Fatalf("column at %v, %v wide", x, w)
	}
	tt.Press(5, 8)
	tt.Release(5, 8)
	if a, z := doc.Selection(); a != 0 || z != 0 {
		t.Fatalf("a click left of the column selects %d–%d, not the start", a, z)
	}
	tt.Press(495, 8)
	tt.Release(495, 8)
	if a, z := doc.Selection(); a != 11 || z != 11 {
		t.Fatalf("a click right of the column selects %d–%d, not the end", a, z)
	}
}

// The top and bottom margins scroll with the text, and the paragraph
// spacing parts paragraphs, lists by their own.
func TestRichTextMarginsAndSpacing(t *testing.T) {
	doc := NewDocument(NewText("one\ntwo\nthree\nfour"))
	doc.SetSelection(9, 9)
	doc.FormatParagraphs(func(st *ParagraphStyle) { st.List = Bullets })
	doc.SetSelection(14, 14)
	doc.FormatParagraphs(func(st *ParagraphStyle) { st.List = Bullets })
	var s Scroll
	th := &Theme{ParagraphSpacing: 1, ListSpacing: 0.5}
	opts := EditorOptions{Font: ui.Font{Size: 20}, Theme: th, Margin: [4]float32{40, 0, 60, 0}, Scroll: &s}
	baseTester(doc, opts)
	y0, y1, y2, y3 := s.TextY(0), s.TextY(4), s.TextY(8), s.TextY(14)
	if !near(y0, 40) {
		t.Fatalf("the text starts at %v, not below the top margin", y0)
	}
	line := y3 - y2 - 10
	if !near(y1-y0, line+20) || !near(y2-y1, line+20) {
		t.Fatalf("paragraphs at %v, %v, %v: no spacing of 1em between them (a line %v)", y0, y1, y2, line)
	}
	if !near(s.Content(), y3+line+60) {
		t.Fatalf("content %v high, not ending in the bottom margin past %v", s.Content(), y3+line)
	}
}

// Up and Down go across the room between paragraphs, however large.
func TestRichTextVerticalAcrossSpacing(t *testing.T) {
	doc := NewDocument(NewText("first\nsecond\nthird"))
	opts := EditorOptions{Theme: &Theme{ParagraphSpacing: 3}}
	tt := baseTester(doc, opts)
	tt.Press(2, 5)
	tt.Release(2, 5)
	tt.Key(0, ui.KeyDown)
	if _, z := doc.Selection(); z < 6 || z > 12 {
		t.Fatalf("Down went to %d, not into the second paragraph", z)
	}
	tt.Key(0, ui.KeyDown)
	if _, z := doc.Selection(); z < 13 {
		t.Fatalf("Down went to %d, not into the third paragraph", z)
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	if _, z := doc.Selection(); z > 5 {
		t.Fatalf("Up went to %d, not back into the first paragraph", z)
	}
}

// Scroll tells where the editor is scrolled, scrolls it, Measure measures
// what is out of view, and the editor paints no bar of its own once
// hidden.
func TestRichTextScrollState(t *testing.T) {
	text := strings.TrimSuffix(strings.Repeat("A line of text\n", 80), "\n")
	doc := NewDocument(NewText(text))
	doc.SetSelection(0, 0)
	var s Scroll
	tt := baseTester(doc, EditorOptions{Scroll: &s, HideScrollbar: true, Measure: true})
	if s.Viewport() != 400 || s.Content() <= 400 {
		t.Fatalf("viewport %v, content %v", s.Viewport(), s.Content())
	}
	tt.Advance(100 * time.Millisecond)
	if _, unknown := s.v.hs.totals(); unknown > 0 {
		t.Fatalf("%d paragraphs out of view still unmeasured", unknown)
	}
	tt.Scroll(250, 200, 0, 120)
	tt.Frame()
	if !near(s.Offset(), 120) {
		t.Fatalf("the wheel scrolled to %v, not 120", s.Offset())
	}
	s.ScrollTo(1e6)
	tt.Frame()
	if !near(s.Offset(), s.Content()-s.Viewport()) {
		t.Fatalf("scrolled to %v, past the end %v", s.Offset(), s.Content()-s.Viewport())
	}
	at := 40 * 15
	s.ScrollToText(at, 10)
	tt.Frame()
	if !near(s.Offset(), s.TextY(at)-10) {
		t.Fatalf("scrolled to %v, not 10 above line 40 at %v", s.Offset(), s.TextY(at))
	}
	img := tt.Image()
	bg := img.At(470, 200)
	for y := 0; y < 400; y++ {
		if img.At(494, y) != bg {
			t.Fatalf("something paints at the right edge, y %d, with the scroll bar hidden", y)
		}
	}
}

// As the column changes width, the text in view stays, and Offset with it.
func TestRichTextScrollKeepsPlaceAcrossWidths(t *testing.T) {
	var b strings.Builder
	for i := range 60 {
		b.WriteString(strings.Repeat("word ", 5+i%7*9))
		b.WriteString("\n")
	}
	doc := NewDocument(NewText(b.String()))
	doc.SetSelection(0, 0)
	var s Scroll
	opts := EditorOptions{MaxWidth: 300, Scroll: &s}
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, opts).Fill()
	}, 500, 400)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	at := doc.Text().paraStart(30)
	s.ScrollToText(at, 0)
	tt.Frame()
	opts.MaxWidth = 460
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !near(s.Offset(), s.TextY(at)) {
		t.Fatalf("offset %v, not the top of paragraph 30 at %v", s.Offset(), s.TextY(at))
	}
}

// A line break starts a line: the caret after it is a line down, at the
// paragraph's left edge, and Up takes it back to the line above.
func TestLineBreakLaysOutALine(t *testing.T) {
	doc := NewDocument(TextOf(Paragraph{Runs: []Run{{Text: "first second"}}}))
	var s Scroll
	doc.SetSelection(0, 0)
	tt := baseTester(doc, EditorOptions{Scroll: &s})
	y0, h, ok := s.CaretLine()
	if !ok {
		t.Fatal("no caret")
	}
	doc.SetSelection(6, 6)
	tt.Frame()
	y1, _, _ := s.CaretLine()
	if y1 < y0+h*0.9 {
		t.Fatalf("caret after the break at %v, first line at %v (height %v)", y1, y0, h)
	}
	tt.Press(2, 5)
	tt.Release(2, 5)
	tt.Key(0, ui.KeyDown)
	if _, z := doc.Selection(); z < 6 {
		t.Fatalf("Down went to %d", z)
	}
	tt.Key(0, ui.KeyUp)
	if _, z := doc.Selection(); z > 5 {
		t.Fatalf("Up went to %d", z)
	}
}

// An item's number counts on past the paragraphs and code under the item
// before it.
func TestListNumberPastBlocksUnderItem(t *testing.T) {
	doc := NewDocument(ParseMarkdown("1. one\n\n    more\n\n    ```\n    x\n    ```\n\n2. two\n\npara\n\n1. again"))
	var s Scroll
	baseTester(doc, EditorOptions{Scroll: &s})
	v := s.v
	got := []int{}
	for p := range v.paras {
		if v.para(p).style.List == Numbers {
			got = append(got, v.listNumber(p))
		}
	}
	if fmt.Sprint(got) != "[1 2 1]" {
		t.Fatalf("numbers %v", got)
	}
}
