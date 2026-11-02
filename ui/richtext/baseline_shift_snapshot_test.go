//go:build snapshot

package richtext

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestBaselineShiftSnapshots(t *testing.T) {
	baselineSnapshots(t, "shifted", func(tag string, s *ui.Span) {
		switch tag {
		case "raise":
			s.BaselineOffset = 9
		case "lower":
			s.BaselineOffset = -6
		}
	})
}
