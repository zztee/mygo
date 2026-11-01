package richtext

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// BenchmarkMeasurementPass measures one background pass on roughly 1MB
// of formatted text, excluding the reset and the visible paragraphs.
func BenchmarkMeasurementPass(b *testing.B) {
	for _, budget := range []time.Duration{time.Millisecond, 4 * time.Millisecond} {
		b.Run(budget.String(), func(b *testing.B) {
			d := NewDocument(longText(8000))
			var s Scroll
			tt := ui.NewTester(func(c *ui.Context) { EditorBase(c, d, EditorOptions{Scroll: &s}).Fill() }, 1000, 800)
			v := s.v
			var measured int
			var longest time.Duration
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				clear(v.paras)
				v.laid = 0
				v.resetHeights()
				v.place(v.viewH)
				_, before := v.hs.totals()
				b.StartTimer()
				start := time.Now()
				v.measure(budget, time.Now)
				took := time.Since(start)
				b.StopTimer()
				_, after := v.hs.totals()
				measured += before - after
				longest = max(longest, took)
			}
			b.ReportMetric(float64(measured)/float64(b.N), "paras/pass")
			b.ReportMetric(float64(longest.Nanoseconds()), "max-ns/pass")
			_ = tt
		})
	}
}
