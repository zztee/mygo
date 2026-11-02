package mygo

import (
	"errors"

	"github.com/egoist/mygo/internal/platform"
)

// CapturePageOptions selects an area of a web page to capture.
type CapturePageOptions struct {
	// Rect is in logical web view coordinates, with its origin at the top left.
	// A zero rectangle captures the visible viewport. A nonzero rectangle
	// must have nonnegative coordinates and positive dimensions.
	Rect Rectangle
	// Width is the output width in pixels, independent of the display scale.
	// Zero uses the display's native scale. The aspect ratio is preserved.
	Width int
}

// CapturePageWithOptions returns a PNG of the selected page area. On macOS
// the area may extend beyond the viewport, without resizing the window.
// On other platforms the options are ignored and CapturePage is used.
// Windows showing native Content only support zero options.
func CapturePageWithOptions(w *Window, o CapturePageOptions) ([]byte, error) {
	r := o.Rect
	if o.Width < 0 || r != (Rectangle{}) && (r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0) {
		return nil, errors.New("mygo: invalid page capture options")
	}
	if o == (CapturePageOptions{}) {
		return w.CapturePage()
	}
	if w.content != nil {
		return nil, errNoPage
	}
	type result struct {
		png []byte
		err error
	}
	ch := make(chan result, 1)
	w.do(func(n platform.Window) {
		cb := func(png []byte, err error) { deliver(ch, result{png, err}) }
		if capturer, ok := n.(platform.PageCapturer); ok {
			capturer.CapturePageWithOptions(platform.CapturePageOptions{Rect: platform.Rect(r), Width: o.Width}, cb)
		} else {
			captureNativePage(n, platform.CapturePageOptions{Rect: platform.Rect(r), Width: o.Width}, cb)
		}
	})
	if w.IsDestroyed() {
		return nil, errDestroyed
	}
	got := await(ch)
	return got.png, got.err
}
