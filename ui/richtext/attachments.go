package richtext

import "github.com/egoist/mygo/ui"

// AttachmentBox returns the box of the attachment at rune i, in the
// editor's own coordinates, the space of its pointer events and IndexAt,
// as the editor lays it out: shrunk to the column when wider, and
// scrolled. An app puts an element of its own over the attachment there,
// as a field over a cell of a table it draws. It reports false when rune
// i holds no attachment, or before the editor is first drawn.
func (s *Scroll) AttachmentBox(i int) (ui.Rect, bool) {
	v := s.v
	if v == nil || len(v.paras) == 0 || v.width <= 0 || i < 0 || i >= v.shown.Len() {
		return ui.Rect{}, false
	}
	if v.shown.AttributesAt(i).Attachment == nil {
		return ui.Rect{}, false
	}
	p := v.shown.paraAt(i)
	pv := v.paraLayout(p)
	ox, oy := v.origin(p)
	for _, o := range pv.layout.Objects() {
		if v.global(p, pv.runeOfUnit(o.Index)) != i {
			continue
		}
		return ui.Rect{X: v.padX + ox + o.Rect.X, Y: v.padY + float32(oy-v.scroll) + o.Rect.Y, W: o.Rect.W, H: o.Rect.H}, true
	}
	return ui.Rect{}, false
}

// pressAttachment tells the app of a press on an attachment's box
// (EditorOptions.OnAttachment), and reports whether it took a press of the
// primary button. The secondary button's opens the context menu on it.
func (v *view) pressAttachment(ev ui.InputEvent) bool {
	if v.opts.OnAttachment == nil && v.opts.ContextMenu == nil {
		return false
	}
	x, y := v.contentPoint(ev.X, ev.Y)
	i, ax, ay, ok := v.attachmentAt(x, y)
	if !ok {
		return false
	}
	if ev.Button == 1 {
		v.menuAt = i + 1
	}
	return v.opts.OnAttachment != nil && v.opts.OnAttachment(i, ax, ay, ev) && ev.Button == 0
}

// attachmentAt returns the attachment whose box holds (x, y) of the
// content, its rune, and where in it the point is, in the attachment's
// own size.
func (v *view) attachmentAt(x float32, y float64) (i int, ax, ay float32, ok bool) {
	if len(v.paras) == 0 {
		return 0, 0, 0, false
	}
	p, _, _ := v.layoutPositionAt(x, y)
	if p < 0 {
		return 0, 0, 0, false
	}
	pv := v.paraLayout(p)
	ox, oy := v.origin(p)
	px, py := x-ox, float32(y-oy)
	for _, o := range pv.layout.Objects() {
		r := o.Rect
		if px < r.X || px >= r.X+r.W || py < r.Y || py >= r.Y+r.H || r.W <= 0 || r.H <= 0 {
			continue
		}
		g := v.global(p, pv.runeOfUnit(o.Index))
		at := v.shown.AttributesAt(g).Attachment
		if at == nil {
			continue
		}
		w, h := at.size()
		sx, sy := float32(1), float32(1)
		if w > 0 && h > 0 {
			sx, sy = w/r.W, h/r.H
		}
		return g, (px - r.X) * sx, (py - r.Y) * sy, true
	}
	return 0, 0, 0, false
}
