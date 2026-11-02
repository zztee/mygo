package richtext

import (
	"github.com/egoist/mygo/ui"
	"unicode/utf8"
)

// inputHook runs an input action in one undo step. Declining an action
// leaves the typing coalescing state as it was before the callback.
func (v *view) inputHook(composition bool, fn func(*Edit) bool) bool {
	d := v.doc
	d.lock()
	defer d.mu.Unlock()
	v.lockedSync()
	v.lockedPublish()
	mode, coalesce := v.mode, v.coalesce
	if composition {
		v.joinStep()
	} else {
		v.mode = newStep
	}
	handled := false
	before := d.version
	v.lockedEdit(func(e *Edit) { handled = fn(e) })
	if handled {
		v.mode, v.coalesce, d.coalesce = newStep, false, false
		if composition && d.version != before {
			v.mode = sameStep
		}
		v.selAnchor, v.caret = d.anchor, d.caret
		v.selVersion = d.selVersion
		v.affinity, v.hasDesired = ui.TextDownstream, false
		v.reveal = true
	} else {
		v.mode, v.coalesce = mode, coalesce
	}
	return handled
}

// textInputHook restores the selection a composition hid before offering
// its committed text. Restoring and replacing it share the composition's
// undo step, including its original attributes and selection direction.
func (v *view) textInputHook(s string, composition bool) bool {
	if v.opts.OnTextInput == nil || s == "" {
		return false
	}
	replaced, reversed := v.composeReplacement, v.composeReversed
	return v.inputHook(composition, func(e *Edit) bool {
		typing := e.d.typing
		restore := composition && replaced.Len() > 0
		if restore {
			_, c := e.Selection()
			e.Replace(c, c, replaced)
			e.FormatParagraphs(c+replaced.Len(), c+replaced.Len(), func(p *ParagraphStyle) { *p = v.composeTail })
			if reversed {
				e.Select(c+replaced.Len(), c)
			} else {
				e.Select(c, c+replaced.Len())
			}
			e.d.typing = typing
		}
		if v.opts.OnTextInput(e, s) {
			return true
		}
		if restore {
			// A declined hook still replaces the restored selection with
			// the committed text, as the native editor would have done.
			a, c := e.Selection()
			a, c = min(a, c), max(a, c)
			e.ReplaceString(a, c, s, e.d.typingAttributes())
			e.Select(a+utf8.RuneCountInString(s), a+utf8.RuneCountInString(s))
		}
		return restore
	})
}

// finishCompose commits text from the input method, without calling the
// hook for provisional marked text or explicit non-composition edits.
func (v *view) finishCompose(s string) {
	v.compose = ""
	v.joinStep()
	if !v.textInputHook(s, true) {
		v.replace(v.caret, v.caret, s)
	}
	v.clearComposeCapture()
}

// TypingAttributes returns the attributes text inserted at the current
// selection would receive, including attributes set by a formatting command.
func (e *Edit) TypingAttributes() Attributes { return e.d.typingAttributes() }

// clearComposeCapture releases the original selection when a composition
// commits, is abandoned, or is superseded by an unrelated input action.
func (v *view) clearComposeCapture() {
	v.composeReplacement, v.composeReversed = Text{}, false
	v.composeTail = ParagraphStyle{}
	v.composePending, v.composePendingRange = false, ui.TextInputRange{}
}

func sameInputRange(a, b ui.TextInputRange) bool {
	return min(a.Start, a.End) == min(b.Start, b.End) && max(a.Start, a.End) == max(b.Start, b.End)
}
