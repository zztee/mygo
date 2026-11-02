package fake

import "github.com/egoist/mygo/internal/platform"

func (w *Window) CapturePageWithOptions(o platform.CapturePageOptions, cb func([]byte, error)) {
	if w.CapturePageHandler != nil {
		w.CapturePageHandler(o, cb)
		return
	}
	w.CapturePage(cb)
}
