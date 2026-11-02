//go:build !darwin

package mygo

import "github.com/egoist/mygo/internal/platform"

func captureNativePage(n platform.Window, _ platform.CapturePageOptions, cb func([]byte, error)) {
	n.CapturePage(cb)
}
