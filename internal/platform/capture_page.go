package platform

// CapturePageOptions describes a page region in logical coordinates and
// its output width in pixels. Zero values keep the viewport and native scale.
type CapturePageOptions struct {
	Rect  Rect
	Width int
}

// PageCapturer is an optional window capability, so apps that do not use
// region snapshots do not link their native configuration code.
type PageCapturer interface {
	CapturePageWithOptions(CapturePageOptions, func(png []byte, err error))
}
