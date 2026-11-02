package richtext

// Presentation is the displayed form of a document and its selection. Typing
// holds explicit typing attributes, or is nil to derive them from the text.
// Text offsets count runes, just as Document.Selection does.
type Presentation struct {
	Text          Text
	Anchor, Caret int
	Typing        *Attributes
}

// SetPresentation changes how the document represents the same content without
// adding a step of undo or discarding undo and redo. For example, an app can
// show Markdown delimiters or replace them with attributed runs. fn maps text,
// selection and explicit typing attributes to the desired representation.
//
// fn runs once now and lazily when undo or redo restores an older version. It
// must preserve content, be idempotent, and accept versions in any previous
// representation. Capture a fixed presentation policy rather than mutable app
// state. The callback runs under the document's lock: keep it short and do not
// call methods of this document from it. History is not traversed on a switch.
// For a live editor, prefer EditorOptions.Presentation: it waits until marked
// input-method text has been committed or cancelled before making the switch.
//
// A switch ends typing coalescing and invalidates existing checkpoints. A nil
// callback removes the policy without changing the current text; future history
// commands then restore their stored representations. SetText removes it too.
func (d *Document) SetPresentation(fn func(Presentation) Presentation) {
	d.lock()
	defer d.mu.Unlock()
	d.setPresentation(fn)
}

// setPresentation applies a policy while the caller holds the document lock.
func (d *Document) setPresentation(fn func(Presentation) Presentation) {
	var shown Presentation
	if fn != nil {
		shown = mapPresentation(fn, Presentation{d.text, d.anchor, d.caret, d.typing})
	}
	d.presentation = fn
	d.presentationKey, d.presentationFromOptions = "", false
	d.presentationEpoch++
	d.presented = d.presented || fn != nil
	d.coalesce = false
	if fn == nil {
		return
	}
	changed := !d.text.Equal(shown.Text)
	if changed {
		old := d.text.Len()
		d.text = shown.Text
		d.changed(0, old, shown.Text.Len())
	}
	if changed || d.anchor != shown.Anchor || d.caret != shown.Caret {
		d.setSelection(shown.Anchor, shown.Caret)
	}
	d.typing = shown.Typing
}

// applyPresentation notices resets of the same document as well as changes of
// the editor's option key. Marked text and pending native replacements keep the
// old policy until they finish.
func (v *view) applyPresentation() {
	if v.compose != "" || v.composePending || v.opts.Presentation == nil && !v.presentationSet {
		return
	}
	d := v.doc
	d.lock()
	defer d.mu.Unlock()
	if v.opts.Presentation != nil {
		// A shared editor may already have installed this policy. Checking the
		// document's key avoids turning its installation into another switch.
		if !d.presentationFromOptions || d.presentationKey != v.opts.PresentationKey {
			d.setPresentation(v.opts.Presentation)
			d.presentationKey, d.presentationFromOptions = v.opts.PresentationKey, true
		}
	} else if d.presentationFromOptions {
		d.setPresentation(nil)
	}
	v.presentationKey, v.presentationSet = v.opts.PresentationKey, v.opts.Presentation != nil
}

// mapPresentation owns the typing attributes on both sides of a callback, so
// neither the callback nor its caller can mutate stored history through them.
func mapPresentation(fn func(Presentation) Presentation, p Presentation) Presentation {
	if p.Typing != nil {
		a := *p.Typing
		p.Typing = &a
	}
	p = fn(p)
	n := p.Text.Len()
	p.Anchor, p.Caret = max(0, min(p.Anchor, n)), max(0, min(p.Caret, n))
	if p.Typing != nil {
		a := *p.Typing
		a.Attachment, a.SoftBreak, a.SoftBreakBefore = nil, false, false
		p.Typing = &a
	}
	return p
}
