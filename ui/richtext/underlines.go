package richtext

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Underline is a temporary mark over [Start, End) in the displayed text,
// counted in runes. A zero Color takes the editor's text color. Wavy draws
// a wave, as spelling errors use; otherwise the line is straight.
type Underline struct {
	Start, End int
	Color      ui.Color
	Wavy       bool
}

func (v *view) paintUnderlines(p *ui.Painter, paragraph int, pv *paraView, x, y float32) {
	if v.opts.Underlines == nil || v.compose != "" && v.shown.paraAt(v.caret) == paragraph {
		return
	}
	start := v.shown.paraStart(paragraph)
	end := start + v.para(paragraph).n
	for _, mark := range v.opts.Underlines(v.shown, paragraph) {
		a, b := max(start, mark.Start), min(end, mark.End)
		if a >= b {
			continue
		}
		color := mark.Color
		if color.A == 0 {
			color = v.look.Text
		}
		for _, r := range pv.layout.SelectionRects(ui.TextInputRange{Start: pv.units[a-start], End: pv.units[b-start]}) {
			left, right, baseline := x+r.X, x+r.X+r.W, y+r.Y+r.H-1
			if !mark.Wavy {
				p.Line(left, baseline, right, baseline, 1, color)
				continue
			}
			paintUnderlineWave(p, left, right, baseline, color)
		}
	}
}

func paintUnderlineWave(p *ui.Painter, left, right, baseline float32, color ui.Color) {
	// Paths are limited to 4096 device pixels. Leave room for stroke padding
	// and keep the phase across chunks so wide, scaled lines stay continuous.
	width := float32(2048) / p.Scale()
	y := func(x float32) float32 {
		return baseline - 0.75*float32(math.Sin(float64((x-left)/6*2*math.Pi)))
	}
	for from := left; from < right; {
		to := min(from+width, right)
		var path ui.Path
		path.MoveTo(from, y(from))
		// Eight points per six-DIP wave keep the curve smooth at every scale.
		n := max(int(math.Ceil(float64((to-from)/0.75))), 1)
		for i := 1; i <= n; i++ {
			x := from + (to-from)*float32(i)/float32(n)
			path.LineTo(x, y(x))
		}
		p.StrokePath(&path, 1, color)
		from = to
	}
}
