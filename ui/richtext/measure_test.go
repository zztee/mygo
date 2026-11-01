package richtext

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func measurementTester() (*ui.Tester, *Scroll, *EditorOptions, **Document) {
	d := NewDocument(NewText(strings.Repeat("A line to measure.\n", 120)))
	var s Scroll
	opts := &EditorOptions{Scroll: &s}
	tt := ui.NewTester(func(c *ui.Context) { EditorBase(c, d, *opts).Fill() }, 500, 100)
	return tt, &s, opts, &d
}

// A fake work clock bounds each pass by completed paragraphs, even when
// one paragraph takes longer than the whole budget.
func TestMeasurementBudget(t *testing.T) {
	_, s, _, _ := measurementTester()
	v := s.v
	_, before := v.hs.totals()
	now := time.Unix(1, 0)
	clock := func() time.Time {
		at := now
		now = now.Add(2 * time.Millisecond)
		return at
	}
	if !v.measure(5*time.Millisecond, clock) {
		t.Fatal("a bounded pass measured the whole document")
	}
	if _, left := v.hs.totals(); before-left != 3 {
		t.Fatalf("measured %d paragraphs, want 3", before-left)
	}
	for range before {
		_, n := v.hs.totals()
		more := v.measure(time.Nanosecond, clock)
		_, left := v.hs.totals()
		if n-left != 1 || more != (left > 0) {
			t.Fatalf("tiny pass: %d to %d unknown, more %v", n, left, more)
		}
		if !more {
			break
		}
	}
	if _, left := v.hs.totals(); left != 0 {
		t.Fatalf("%d paragraphs never measured", left)
	}
	if v.measure(time.Second, func() time.Time { t.Fatal("completed layout used the clock"); return time.Time{} }) {
		t.Fatal("completed layout requested another pass")
	}
}

// Repaints and input before the interval cannot drain the background
// work. Its timer finishes the exact height without another view build.
func TestMeasurementInterval(t *testing.T) {
	tt, s, opts, _ := measurementTester()
	_, before := s.v.hs.totals()
	opts.Measure, opts.MeasureBudget, opts.MeasureInterval = true, time.Hour, 50*time.Millisecond
	s.v.measuredAt = tt.Now()
	tt.Frame()
	for range 3 {
		tt.Frame()
	}
	tt.Advance(49 * time.Millisecond)
	if _, left := s.v.hs.totals(); left != before {
		t.Fatalf("measured before the interval: %d to %d unknown", before, left)
	}
	tt.Advance(time.Millisecond)
	if _, left := s.v.hs.totals(); left != 0 {
		t.Fatalf("timer left %d paragraphs unmeasured", left)
	}
	if s.Content() <= s.Viewport() {
		t.Fatal("measurement lost the document's height")
	}
}

func TestMeasurementInputAndReplacement(t *testing.T) {
	tt, s, opts, d := measurementTester()
	opts.Measure, opts.MeasureBudget, opts.MeasureInterval = true, time.Hour, time.Second
	s.v.measuredAt = tt.Now()
	tt.Frame()
	tt.Press(10, 10)
	tt.Release(10, 10)
	(*d).SetSelection(0, 0)
	tt.Frame()
	tt.Type("edited ")
	if !strings.HasPrefix((*d).Text().String(), "edited ") {
		t.Fatalf("input waited for background measurement: %q", (*d).Text().String())
	}
	s.ScrollToText((*d).Text().paraStart(60), 0)
	tt.Frame()
	if s.Offset() <= 0 || s.v.first < 50 {
		t.Fatal("scrolling waited for background measurement")
	}
	old := *d
	*d = NewDocument(NewText(strings.Repeat("The replacement has different text.\n", 40)))
	tt.Frame()
	tt.Advance(time.Second)
	if s.v.doc != *d || s.v.shown.String() != (*d).Text().String() {
		t.Fatal("pending measurement restored the old document")
	}
	if !strings.HasPrefix(old.Text().String(), "edited ") {
		t.Fatal("measurement changed the document left behind")
	}
	if _, left := s.v.hs.totals(); left != 0 {
		t.Fatalf("replacement left %d paragraphs unmeasured", left)
	}
}

func TestMeasurementDisabledAndResumed(t *testing.T) {
	tt, s, opts, _ := measurementTester()
	_, before := s.v.hs.totals()
	opts.Measure, opts.MeasureBudget, opts.MeasureInterval = true, time.Hour, time.Second
	s.v.measuredAt = tt.Now()
	tt.Frame()
	opts.Measure = false
	tt.Frame()
	tt.Advance(2 * time.Second)
	if _, left := s.v.hs.totals(); left != before {
		t.Fatalf("disabled measurement continued: %d to %d unknown", before, left)
	}
	opts.Measure = true
	tt.Frame()
	if _, left := s.v.hs.totals(); left != 0 {
		t.Fatalf("resumed measurement left %d unknown", left)
	}
}

// A changed interval takes effect immediately, rather than retaining
// the deadline of the preceding settings.
func TestMeasurementIntervalChanged(t *testing.T) {
	tt, s, opts, _ := measurementTester()
	opts.Measure, opts.MeasureBudget, opts.MeasureInterval = true, time.Hour, time.Second
	s.v.measuredAt = tt.Now()
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	opts.MeasureInterval = 10 * time.Millisecond
	tt.Frame()
	if _, left := s.v.hs.totals(); left != 0 {
		t.Fatalf("shorter interval left %d unknown", left)
	}
}

// Nonpositive intervals request the next display frame, including the
// first pass after measurement was disabled.
func TestMeasurementNegativeInterval(t *testing.T) {
	for _, mode := range []string{"initial", "resumed"} {
		t.Run(mode, func(t *testing.T) {
			tt, s, opts, d := measurementTester()
			opts.MeasureBudget = time.Hour
			if mode == "resumed" {
				opts.Measure = true
				tt.Frame()
				opts.Measure = false
				(*d).SetText(NewText(strings.Repeat("A fresh paragraph.\n", 100)))
				tt.Frame()
			}
			opts.Measure, opts.MeasureInterval = true, -time.Millisecond
			tt.Frame()
			if _, left := s.v.hs.totals(); left != 0 {
				t.Fatalf("negative interval left %d paragraphs unmeasured", left)
			}
		})
	}
}
