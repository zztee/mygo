//go:build !js

package mygo

import (
	"bytes"
	"errors"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/ui"
)

func TestCapturePageWithOptions(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Width: 640, Height: 480})
	want := CapturePageOptions{Rect: Rectangle{X: 7, Y: 19, Width: 640, Height: 4000}, Width: 1920}
	calls := 0
	failure := errors.New("snapshot failed")
	onMain(func() {
		fw.CapturePageHandler = func(o platform.CapturePageOptions, cb func([]byte, error)) {
			calls++
			if !isMainThread() {
				t.Error("capture ran off the main thread")
			}
			if o.Rect != platform.Rect(want.Rect) || o.Width != want.Width {
				t.Errorf("options=%+v", o)
			}
			// Reply from a later event, so the main-thread caller must pump events.
			postMain(func() { cb([]byte("region"), failure) })
		}
	})
	for _, main := range []bool{false, true} {
		capture := func() {
			got, err := CapturePageWithOptions(w, want)
			if !bytes.Equal(got, []byte("region")) || !errors.Is(err, failure) {
				t.Errorf("capture=%q, %v", got, err)
			}
		}
		if main {
			onMain(capture)
		} else {
			capture()
		}
	}
	onMain(func() {
		if calls != 2 {
			t.Errorf("captures=%d", calls)
		}
	})
}

func TestCapturePageOptionsValidationAndDefaults(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	onMain(func() {
		fw.CapturePageHandler = func(platform.CapturePageOptions, func([]byte, error)) { t.Error("unexpected region capture") }
	})
	for _, o := range []CapturePageOptions{
		{Width: -1}, {Rect: Rectangle{X: -1, Width: 1, Height: 1}}, {Rect: Rectangle{Y: -1, Width: 1, Height: 1}},
		{Rect: Rectangle{Width: 1}}, {Rect: Rectangle{Height: 1}}, {Rect: Rectangle{Width: -1, Height: 1}},
	} {
		if _, err := CapturePageWithOptions(w, o); err == nil {
			t.Errorf("accepted %+v", o)
		}
	}
	got, err := CapturePageWithOptions(w, CapturePageOptions{})
	if err != nil || string(got) != "png" {
		t.Fatalf("zero options=%q, %v", got, err)
	}
	w.Destroy()
	if _, err := CapturePageWithOptions(w, CapturePageOptions{Width: 640}); !errors.Is(err, errDestroyed) {
		t.Fatalf("destroyed=%v", err)
	}
}

func TestCapturePageOptionsNativeContent(t *testing.T) {
	w, _, _ := contentWindow(t, func(c *ui.Context) { c.Root().Background(ui.RGB(255, 0, 0)) })
	if _, err := CapturePageWithOptions(w, CapturePageOptions{Width: 640}); !errors.Is(err, errNoPage) {
		t.Fatalf("native options=%v", err)
	}
	got, err := CapturePageWithOptions(w, CapturePageOptions{})
	if err != nil || !bytes.HasPrefix(got, []byte("\x89PNG")) {
		t.Fatalf("native default=%d bytes, %v", len(got), err)
	}
}

// viewportOnlyWindow deliberately does not expose the optional capability.
type viewportOnlyWindow struct{ platform.Window }

func TestCapturePageOptionsBackendFallback(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	onMain(func() { w.native = viewportOnlyWindow{fw} })
	got, err := CapturePageWithOptions(w, CapturePageOptions{Rect: Rectangle{Width: 640, Height: 4000}, Width: 1280})
	if err != nil || string(got) != "png" {
		t.Fatalf("fallback=%q, %v", got, err)
	}
}
