package richtext

import "github.com/egoist/mygo/ui"

// pressBelow uses the same rectangle as paint for the room under a
// paragraph. Its gaps, the column margins and the scrollbar stay the
// editor's, and a secondary press still opens its context menu.
func (v *view) pressBelow(ev ui.InputEvent) bool {
	if v.opts.OnBelow == nil || len(v.paras) == 0 {
		return false
	}
	x, y := v.contentPoint(ev.X, ev.Y)
	if x < 0 || x >= v.width {
		return false
	}
	p, _, _ := v.layoutPositionAt(x, y)
	if p < 0 {
		return false
	}
	pv := v.paraLayout(p)
	if pv.below <= 0 {
		return false
	}
	_, gap := v.gaps(p, v.para(p).style)
	top := v.hs.top(p) + float64(pv.h-gap-pv.below)
	if y < top || y >= top+float64(pv.below) {
		return false
	}
	return v.opts.OnBelow(p, x, float32(y-top), ev) && ev.Button == 0
}
