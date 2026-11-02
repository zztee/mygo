//go:build windows && (amd64 || arm64)

package text

import (
	"math"
	"testing"
)

func TestDirectWriteBaselineLigatureBoundary(t *testing.T) {
	s := Shared()
	style := Style{Family: "Calibri", Size: 24}
	control := s.Layout(Params{Text: "fi", Style: style})
	if len(control.Lines) != 1 || len(control.Lines[0].Glyphs) != 1 || control.Lines[0].Glyphs[0].Runes != 2 {
		t.Fatalf("Calibri control did not form the required fi ligature: %+v", control.Lines)
	}
	for _, offsets := range [][2]float32{{0, 8}, {8, 0}, {8, -6}} {
		l := s.Layout(Params{Text: "fi", Style: style, Spans: EncodeSpans([]Span{{End: 1, BaselineOffset: offsets[0]}, {End: 2, BaselineOffset: offsets[1]}})})
		a, al := baselineGlyph(t, l, 0)
		b, bl := baselineGlyph(t, l, 1)
		want := offsets[1] - offsets[0]
		if math.Abs(float64(a.Y-al.Baseline-(b.Y-bl.Baseline)-want)) > .01 {
			t.Fatalf("DirectWrite ligature crossed baseline boundary: %+v %+v", a, b)
		}
	}
}
