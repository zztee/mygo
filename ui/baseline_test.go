package ui

import (
	"github.com/egoist/mygo/internal/text"
	"math"
	"testing"
)

func TestSpanBaselineOffsetObjectsAndDecorations(t *testing.T) {
	for _, offset := range []float32{-30, 30} {
		l := ShapeRichText([]Span{{Text: "a"}, {Text: "￼", ObjectWidth: 30, ObjectHeight: 24, ObjectDescent: 4, BaselineOffset: offset}, {Text: "b"}}, Font{Size: 20}, 0)
		objects := l.Objects()
		if len(objects) != 1 {
			t.Fatalf("objects %+v", objects)
		}
		line := l.l.Lines[0]
		box := objects[0].Rect
		if math.Abs(float64(box.Y-(line.Baseline-offset-20))) > .01 || box.Y < -.01 || box.Y+box.H > line.Height+.01 {
			t.Fatalf("offset %v: object %+v, line %+v", offset, box, line)
		}
		if l.Caret(2).X < box.X+box.W-.1 {
			t.Fatal("object offset moved the caret horizontally")
		}
	}
	for _, decoration := range []text.Decoration{text.Underline, text.Strikethrough} {
		shape := func(offset float32) *TextLayout {
			return ShapeRichText([]Span{{Text: "H"}, {Text: "2", Size: 16, BaselineOffset: offset}, {Text: "O"}}, Font{Size: 24}, 0)
		}
		original, shifted := shape(0), shape(9)
		var i, j int
		for k, g := range shifted.l.Lines[0].Glyphs {
			if g.Cluster == 1 {
				i, j = k, k+1
			}
		}
		before := text.Shared().Decorate(original.l, 0, i, j, 0, 0, 1, decoration, 1)
		after := text.Shared().Decorate(shifted.l, 0, i, j, 0, 0, 1, decoration, 1)
		if len(before) == 0 || len(before) != len(after) {
			t.Fatalf("decorations %+v %+v", before, after)
		}
		// Font rasterizers round decoration positions to pixels.
		want := shifted.l.Lines[0].Baseline - original.l.Lines[0].Baseline - 9
		if math.Abs(float64(after[0].Top-before[0].Top-want)) > 1 {
			t.Fatalf("decoration stayed on the old baseline: %+v -> %+v, want shift %v", before, after, want)
		}
	}
}

func TestSpanBaselineOffsetCacheAndUTF16(t *testing.T) {
	var sc spanCache
	spans := []Span{{Text: "😀"}, {Text: "x", Size: 14, BaselineOffset: 6}, {Text: "end"}}
	sc.update(spans)
	old := sc.key
	spans[1].BaselineOffset = -6
	sc.update(spans)
	if old == sc.key {
		t.Fatal("changing baseline offset reused the old span cache")
	}
	l := ShapeRichText(spans, Font{Size: 22}, 0)
	if l.units[1] != 2 || l.units[2] != 3 {
		t.Fatalf("UTF-16 units %v", l.units)
	}
	for i := 0; i <= 5; i++ {
		if l.Caret(l.units[i]).H <= 0 {
			t.Fatalf("empty caret at rune %d", i)
		}
	}
	if rects := l.SelectionRects(TextInputRange{Start: 2, End: 3}); len(rects) == 0 {
		t.Fatal("shifted text is not selectable")
	}
}

func TestSpanBaselineOffsetPaintsAtItsBaseline(t *testing.T) {
	for _, offset := range []float32{-8, 8} {
		tt := coreNewTester(func(c *context) {
			coreRichText(c, Span{Text: "H"}, Span{Text: "H", BaselineOffset: offset}, Span{Text: "H"}).FontSize(24)
		}, 200, 100)
		gs := tt.h.last.Glyphs
		if len(gs) != 3 {
			t.Fatalf("painted %d glyphs, want 3", len(gs))
		}
		if math.Abs(float64(gs[1].Y-gs[0].Y+offset)) > .1 || gs[2].Y != gs[0].Y {
			t.Fatalf("offset %v: painted y=%v,%v,%v", offset, gs[0].Y, gs[1].Y, gs[2].Y)
		}
	}
}

func TestSpanBaselineOffsetInheritedDecoration(t *testing.T) {
	l := ShapeRichText([]Span{{Text: "H"}, {Text: "H", BaselineOffset: 9}, {Text: "H"}}, Font{Size: 24}, 0)
	for _, d := range []text.Decoration{text.Underline, text.Strikethrough} {
		strokes := text.Shared().Decorate(l.l, 0, 0, 3, 0, 0, 1, d, 1)
		if len(strokes) != 3 {
			t.Fatalf("inherited decoration has %d strokes, want 3: %+v", len(strokes), strokes)
		}
		if math.Abs(float64(strokes[1].Top-strokes[0].Top+9)) > 1 || strokes[2].Top != strokes[0].Top {
			t.Fatalf("inherited decoration did not follow spans: %+v", strokes)
		}
	}
}

func TestSpanBaselineOffsetTextLine(t *testing.T) {
	l := text.Shared().Layout(text.Params{Text: "HHH", Style: text.Style{Size: 24}, Spans: text.EncodeSpans([]text.Span{{End: 1}, {End: 2, BaselineOffset: 8}, {End: 3}})})
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(180, 100).Draw(func(p *Painter, r Rect) { p.TextLine(l, 0, 50, func(int) Color { return RGB(0, 0, 0) }) })
	}, 200, 100)
	gs := tt.h.last.Glyphs
	if len(gs) != 3 || gs[1].Y-gs[0].Y != -8 || gs[2].Y != gs[0].Y {
		t.Fatalf("TextLine ignored baseline offsets: %+v", gs)
	}
}

func TestSpanBaselineOffsetRoundedBackground(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	tt := coreNewTester(func(c *context) {
		coreRichText(c, Span{Text: "H", Background: red, Radius: 3}, Span{Text: "H", Background: blue, Radius: 3, BaselineOffset: 9}).FontSize(24)
	}, 200, 100)
	var a, b Rect
	var foundA, foundB bool
	for _, op := range tt.h.last.Ops {
		if op.Color == red.scene() {
			a = Rect{op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H}
			foundA = true
		}
		if op.Color == blue.scene() {
			b = Rect{op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H}
			foundB = true
		}
	}
	if !foundA || !foundB || math.Abs(float64(b.Y-a.Y+9)) > .1 || a.H != b.H {
		t.Fatalf("rounded backgrounds did not follow the baseline: %+v %+v (%v %v)", a, b, foundA, foundB)
	}
}
