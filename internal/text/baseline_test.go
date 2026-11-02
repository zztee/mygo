package text

import (
	"math"
	"slices"
	"strings"
	"testing"
)

func baselineGlyph(t *testing.T, l *Layout, at int) (Glyph, Line) {
	t.Helper()
	for _, line := range l.Lines {
		for _, g := range line.Glyphs {
			if g.Cluster == at {
				return g, line
			}
		}
	}
	t.Fatalf("no glyph at rune %d in %q", at, l.Params.Text)
	return Glyph{}, Line{}
}

func TestBaselineOffsetEncoding(t *testing.T) {
	spans := []Span{{End: 2, Size: 17, BaselineOffset: 8}, {End: 5, BaselineOffset: -6}, {End: 6}}
	encoded := EncodeSpans(spans)
	if got := DecodeSpans(encoded); !slices.Equal(got, spans) {
		t.Fatalf("decoded %+v", got)
	}
	other := slices.Clone(spans)
	other[0].BaselineOffset = 0
	if encoded == EncodeSpans(other) {
		t.Fatal("baseline offsets do not participate in the layout cache key")
	}
}

func TestBaselineOffsetGlyphsAndExtents(t *testing.T) {
	spans := []Span{{End: 1}, {End: 2, Size: 16, BaselineOffset: 40}, {End: 3}, {End: 4, Size: 16, BaselineOffset: -35}, {End: 5}}
	params := Params{Text: "a2b3c", Style: Style{Size: 24}, Spans: EncodeSpans(spans)}
	shifted := Shared().Layout(params)
	plainSpans := slices.Clone(spans)
	for i := range plainSpans {
		plainSpans[i].BaselineOffset = 0
	}
	params.Spans = EncodeSpans(plainSpans)
	plain := Shared().Layout(params)
	if shifted.Width != plain.Width || len(shifted.Lines) != 1 {
		t.Fatalf("offset changed advances: %v vs %v", shifted.Width, plain.Width)
	}
	for at, offset := range []float32{0, 40, 0, -35, 0} {
		g, line := baselineGlyph(t, shifted, at)
		original, baseLine := baselineGlyph(t, plain, at)
		if math.Abs(float64(g.Y-line.Baseline-(original.Y-baseLine.Baseline)+offset)) > .01 {
			t.Errorf("rune %d baseline offset %v", at, g.Y-line.Baseline)
		}
		if g.X != original.X || g.Advance != original.Advance {
			t.Errorf("rune %d moved horizontally", at)
		}
		if g.Font != nil && (g.Y-g.Font.Ascent < line.Y-.01 || g.Y+g.Font.Descent > line.Y+line.Height+.01) {
			t.Errorf("rune %d outside line: %+v in %+v", at, g, line)
		}
	}
	if shifted.Height <= plain.Height {
		t.Fatal("the line did not grow around large offsets")
	}
}

func TestBaselineOffsetWrapParagraphsAndRTL(t *testing.T) {
	source := strings.Repeat("abcd ", 12) + "\nאבג xyz"
	end := len([]rune(source))
	spans := []Span{{End: 6}, {End: end, Size: 18, BaselineOffset: -12}}
	params := Params{Text: source, Style: Style{Size: 24}, Width: 115, Spans: EncodeSpans(spans)}
	shifted := Shared().Layout(params)
	spans[1].BaselineOffset = 0
	params.Spans = EncodeSpans(spans)
	plain := Shared().Layout(params)
	if len(shifted.Lines) < 3 || len(shifted.Lines) != len(plain.Lines) {
		t.Fatalf("wrap changed: %d vs %d", len(shifted.Lines), len(plain.Lines))
	}
	for i, line := range shifted.Lines {
		if line.Start != plain.Lines[i].Start || line.End != plain.Lines[i].End || line.Width != plain.Lines[i].Width {
			t.Errorf("line %d wrapping changed", i)
		}
		for _, g := range line.Glyphs {
			original, ol := baselineGlyph(t, plain, g.Cluster)
			want := float32(0)
			if g.Cluster >= 6 {
				want = 12
			}
			if math.Abs(float64((g.Y-line.Baseline)-(original.Y-ol.Baseline)-want)) > .01 {
				t.Errorf("rune %d has wrong offset across wrapping/newline/RTL", g.Cluster)
			}
		}
	}
}

func TestBaselineOffsetLigatureBoundary(t *testing.T) {
	for _, family := range []string{"Times", "Helvetica", "Times New Roman, Times"} {
		t.Run(family, func(t *testing.T) {
			for _, offsets := range [][2]float32{{0, 8}, {8, 0}, {8, -6}} {
				l := Shared().Layout(Params{Text: "fi", Style: Style{Family: family, Size: 24}, Spans: EncodeSpans([]Span{{End: 1, BaselineOffset: offsets[0]}, {End: 2, BaselineOffset: offsets[1]}})})
				a, al := baselineGlyph(t, l, 0)
				b, bl := baselineGlyph(t, l, 1)
				want := offsets[1] - offsets[0]
				if math.Abs(float64((a.Y-al.Baseline)-(b.Y-bl.Baseline)-want)) > .01 {
					t.Fatalf("ligature crosses baseline boundary: %+v %+v", a, b)
				}
			}
		})
	}
}

func baselineDecorationLayout(n int) *Layout {
	spans := make([]Span, n)
	for i := range spans {
		spans[i] = Span{End: i + 1, BaselineOffset: float32(i%2) * 8}
	}
	return Shared().Layout(Params{Text: strings.Repeat("H", n), Style: Style{Size: 18}, Spans: EncodeSpans(spans)})
}

func TestBaselineOffsetLongInheritedDecoration(t *testing.T) {
	l := baselineDecorationLayout(512)
	strokes := Shared().Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0, 1, Underline, 1)
	if len(strokes) != 512 {
		t.Fatalf("long decorated text has %d strokes", len(strokes))
	}
	for i := 1; i < len(strokes); i += 2 {
		if math.Abs(float64(strokes[i].Top-strokes[i-1].Top+8)) > 1 {
			t.Fatalf("wrong baseline at stroke %d", i)
		}
	}
}
