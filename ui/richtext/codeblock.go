package richtext

import "github.com/egoist/mygo/ui"

// CodeBlockBox returns the box of the block of code holding rune i, the
// paragraphs of code next to each other that paint one background, in the
// editor's own coordinates, as AttachmentBox has them, and where its
// language label is at its top right (Theme.CodeLanguageLabel), an empty
// rect without one. An app puts a control of its own on the block there,
// as a button that copies its code. It reports false when rune i is not
// in a block of code, or before the editor is first drawn.
func (s *Scroll) CodeBlockBox(i int) (box, label ui.Rect, ok bool) {
	v := s.v
	if v == nil || len(v.paras) == 0 || v.width <= 0 || i < 0 || i > v.shown.Len() {
		return ui.Rect{}, ui.Rect{}, false
	}
	p := v.shown.paraAt(i)
	st := v.para(p).style
	if !st.CodeBlock {
		return ui.Rect{}, ui.Rect{}, false
	}
	first, last := p, p
	for first > 0 && v.para(first-1).style.CodeBlock {
		first--
	}
	for last+1 < len(v.paras) && v.para(last+1).style.CodeBlock {
		last++
	}
	look := &v.look
	fst := v.para(first).style
	ga, _ := v.gaps(first, fst)
	top := v.hs.top(first) + float64(ga)
	lst := v.para(last).style
	pv := v.paraLayout(last)
	_, gb := v.gaps(last, lst)
	bottom := v.hs.top(last) + float64(pv.h-gb-pv.below)
	indent := float32(fst.Indent) * look.indent
	box = ui.Rect{X: v.padX + indent, Y: v.padY + float32(top-v.scroll), W: v.width - indent, H: float32(bottom - top)}
	if look.CodeLanguageLabel && fst.Language != "" {
		// Where paintBlock puts it.
		size := max(look.size*0.62, 11)
		if lay := ui.ShapeText(fst.Language, ui.Font{Family: look.CodeFont, Size: size}, 0); lay != nil {
			lw, lh := lay.Size()
			label = ui.Rect{X: box.X + box.W - lw - look.size*0.45, Y: box.Y + look.size*0.22, W: lw, H: lh}
		}
	}
	return box, label, true
}
