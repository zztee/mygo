package text

import (
	"fmt"
	"testing"
)

func BenchmarkBaselineDecorations(b *testing.B) {
	for _, n := range []int{32, 128, 512} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			l := baselineDecorationLayout(n)
			b.ReportAllocs()
			for b.Loop() {
				Shared().Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0, 1, Underline, 1)
			}
		})
	}
}
