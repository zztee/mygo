//go:build darwin

package mygo

import (
	"github.com/egoist/mygo/internal/darwin"
	"github.com/egoist/mygo/internal/platform"
)

func captureNativePage(n platform.Window, o platform.CapturePageOptions, cb func([]byte, error)) {
	darwin.CapturePageWithOptions(n, o, cb)
}
