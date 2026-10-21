package richtext

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// longText returns n paragraphs of formatted text: a heading every
// fiftieth, items of lists, and runs of bold, italic and links.
func longText(n int) Text {
	paras := make([]Paragraph, n)
	for i := range paras {
		st := ParagraphStyle{}
		switch {
		case i%50 == 0:
			st.Heading = 2
		case i%7 == 3:
			st.List = Bullets
		}
		paras[i] = Paragraph{Style: st, Runs: []Run{
			{Text: fmt.Sprintf("Paragraph %d has ", i)},
			{Text: "bold words", Attributes: Attributes{Weight: 700}},
			{Text: ", an "},
			{Text: "italic one", Attributes: Attributes{Italic: true}},
			{Text: " and "},
			{Text: "a link", Attributes: Attributes{Link: "https://example.com"}},
			{Text: " in a line long enough to wrap in a narrow window."},
		}}
	}
	return TextOf(paras...)
}

// benchParagraphs are the sizes of the documents of the benchmarks.
var benchParagraphs = []int{1000, 10000, 100000}

// BenchmarkTypeFrame types a character in the middle of a document and
// draws the frame that shows it, as typing does.
func BenchmarkTypeFrame(b *testing.B) {
	for _, n := range benchParagraphs {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			doc := NewDocument(longText(n))
			tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc).Fill() }, 1000, 800)
			tt.Press(500, 400)
			tt.Release(500, 400)
			mid := doc.Text().Len() / 2
			doc.SetSelection(mid, mid)
			tt.Frame()
			b.ReportAllocs()
			for b.Loop() {
				tt.Type("x")
			}
		})
	}
}

// BenchmarkTypeFrameBelow is BenchmarkTypeFrame with an EditorOptions.Below
// that the paragraphs in view are asked about each frame, and that adds
// room under the one typed in: what asking it costs, where each answer
// needs where its paragraph starts.
func BenchmarkTypeFrameBelow(b *testing.B) {
	for _, n := range benchParagraphs {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			doc := NewDocument(longText(n))
			draw := func(*ui.Painter, ui.Rect) {}
			mid := doc.Text().Len() / 2
			below := func(t Text, p int, st ParagraphStyle) (float32, func(*ui.Painter, ui.Rect)) {
				if start := t.ParagraphStart(p); start > mid || mid-start > 200 {
					return 0, nil
				}
				return 40, draw
			}
			tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc, EditorOptions{Below: below}).Fill() }, 1000, 800)
			tt.Press(500, 400)
			tt.Release(500, 400)
			doc.SetSelection(mid, mid)
			tt.Frame()
			b.ReportAllocs()
			for b.Loop() {
				tt.Type("x")
			}
		})
	}
}

// BenchmarkReplace replaces a rune in the middle of a document's text,
// the edit a keystroke makes, without drawing.
func BenchmarkReplace(b *testing.B) {
	for _, n := range benchParagraphs {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			t := longText(n)
			mid := t.Len() / 2
			b.ReportAllocs()
			for b.Loop() {
				t = t.ReplaceString(mid, mid+1, "y", Attributes{Weight: 700})
			}
		})
	}
}

// BenchmarkFormat makes a thousand runes bold in the middle of a document.
func BenchmarkFormat(b *testing.B) {
	for _, n := range benchParagraphs {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			t := longText(n)
			mid := t.Len() / 2
			on := false
			b.ReportAllocs()
			for b.Loop() {
				on = !on
				t = t.Format(mid, mid+1000, func(a *Attributes) { a.Italic = on })
			}
		})
	}
}

// BenchmarkScrollFrame scrolls a long document by a wheel's step a frame.
func BenchmarkScrollFrame(b *testing.B) {
	doc := NewDocument(longText(100000))
	tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc).Fill() }, 1000, 800)
	tt.Frame()
	b.ReportAllocs()
	for b.Loop() {
		tt.Scroll(500, 400, 0, 120)
	}
}

// BenchmarkScrollFrameBelow is BenchmarkScrollFrame with an
// EditorOptions.Below asked about each paragraph scrolled into view.
func BenchmarkScrollFrameBelow(b *testing.B) {
	doc := NewDocument(longText(100000))
	below := func(Text, int, ParagraphStyle) (float32, func(*ui.Painter, ui.Rect)) { return 0, nil }
	tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc, EditorOptions{Below: below}).Fill() }, 1000, 800)
	tt.Frame()
	b.ReportAllocs()
	for b.Loop() {
		tt.Scroll(500, 400, 0, 120)
	}
}

// BenchmarkParagraphs reads every paragraph of a document, as saving it
// to Markdown does.
func BenchmarkParagraphs(b *testing.B) {
	for _, n := range benchParagraphs {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			t := longText(n)
			b.ReportAllocs()
			for b.Loop() {
				for range t.Paragraphs() {
				}
			}
		})
	}
}
