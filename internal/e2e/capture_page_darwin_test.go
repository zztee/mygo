//go:build darwin

package e2e

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/wait"
)

// TestCapturePageRegion checks real WebKit rasterization outside the original
// viewport, exact output pixels and restoration of both window and view size.
func TestCapturePageRegion(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 640, Height: 480, UseContentSize: true, X: -20000, Y: -20000, Hidden: true, SkipTaskbar: true, DisableShadow: true})
	w.SetOpacity(0)
	w.ShowInactive()
	loaded := make(chan struct{}, 1)
	w.Page().OnDidFinishLoad(func() {
		select {
		case loaded <- struct{}{}:
		default:
		}
	})
	w.Page().LoadHTML(`<!doctype html><style>html,body{margin:0;width:640px}section{height:1200px}</style><section style="background:#ff0000"></section><section style="background:#00ff00"></section><section style="background:#0000ff"></section>`, "")
	select {
	case <-loaded:
	case <-time.After(wait.Guard):
		t.Fatal("page did not load")
	}
	if _, err := w.Page().Eval(`new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Page().Eval(`document.documentElement.style.scrollBehavior="smooth"; window.scrollTo({top:2000, behavior:"instant"}); 0`); err != nil {
		t.Fatal(err)
	}
	beforeScroll, err := mygo.EvalAs[int](w.Page(), "scrollY")
	if err != nil || beforeScroll != 2000 {
		t.Fatalf("initial scroll=%d, %v", beforeScroll, err)
	}
	before := w.ContentBounds()
	viewport, err := mygo.EvalAs[int](w.Page(), "innerHeight")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		rect  mygo.Rectangle
		width int
	}{
		{"normal", mygo.Rectangle{Width: 640, Height: 3600}, 640},
		{"high", mygo.Rectangle{Width: 640, Height: 3600}, 1920},
		{"best", mygo.Rectangle{Width: 640, Height: 3600}, 2560},
		{"fractional", mygo.Rectangle{X: 50, Y: 3200, Width: 300, Height: 300}, 301},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := mygo.CapturePageWithOptions(w, mygo.CapturePageOptions{Rect: tt.rect, Width: tt.width})
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			wantHeight := tt.rect.Height * tt.width / tt.rect.Width
			if b.Dx() != tt.width || b.Dy() != wantHeight {
				t.Fatalf("size=%v, want %dx%d", b, tt.width, wantHeight)
			}
			colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
			if tt.name == "fractional" {
				colors = colors[2:]
			}
			for i, want := range colors {
				got := color.RGBAModel.Convert(img.At(b.Dx()/2, b.Dy()*(2*i+1)/(2*len(colors)))).(color.RGBA)
				if got != want {
					t.Errorf("band %d=%v, want %v", i, got, want)
				}
			}
			if scroll, err := mygo.EvalAs[int](w.Page(), "scrollY"); err != nil || scroll != beforeScroll {
				t.Errorf("scroll=%d, want %d: %v", scroll, beforeScroll, err)
			}
			if after := w.ContentBounds(); before != after {
				t.Errorf("window resized: %v -> %v", before, after)
			}
			if height, err := mygo.EvalAs[int](w.Page(), "innerHeight"); err != nil || height != viewport {
				t.Errorf("viewport=%d, want %d: %v", height, viewport, err)
			}
		})
	}
}

// TestCapturePageHiddenRegion never shows the window. Snapshot restoration
// must finish without animation frames, including a second request afterward.
func TestCapturePageHiddenRegion(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 640, Height: 480, UseContentSize: true, X: -20000, Y: -20000, Hidden: true, SkipTaskbar: true, DisableShadow: true})
	loaded := make(chan struct{}, 1)
	w.Page().OnDidFinishLoad(func() {
		select {
		case loaded <- struct{}{}:
		default:
		}
	})
	w.Page().LoadHTML(`<!doctype html><style>html,body{margin:0;width:640px;scroll-behavior:smooth}section{height:1200px}</style><section style="background:#ff0000"></section><section style="background:#00ff00"></section><section style="background:#0000ff"></section>`, "")
	select {
	case <-loaded:
	case <-time.After(wait.Guard):
		t.Fatal("hidden page did not load")
	}
	if _, err := w.Page().Eval(`window.scrollTo({top:2000,behavior:"instant"}); 0`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		data []byte
		err  error
	}
	for range 2 {
		done := make(chan result, 1)
		go func() {
			data, err := mygo.CapturePageWithOptions(w, mygo.CapturePageOptions{Rect: mygo.Rectangle{Width: 640, Height: 3600}, Width: 640})
			done <- result{data, err}
		}()
		var got result
		select {
		case got = <-done:
		case <-time.After(wait.Guard):
			t.Fatal("hidden capture did not finish")
		}
		if got.err != nil {
			t.Fatal(got.err)
		}
		img, err := png.Decode(bytes.NewReader(got.data))
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != 640 || b.Dy() != 3600 {
			t.Fatalf("hidden size=%v", b)
		}
		for i, want := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
			if got := color.RGBAModel.Convert(img.At(320, 600+i*1200)); got != want {
				t.Errorf("hidden band %d=%v, want %v", i, got, want)
			}
		}
		if scroll, err := mygo.EvalAs[int](w.Page(), "scrollY"); err != nil || scroll != 2000 {
			t.Errorf("hidden scroll=%d, %v", scroll, err)
		}
		if height, err := mygo.EvalAs[int](w.Page(), "innerHeight"); err != nil || height != 480 {
			t.Errorf("hidden viewport=%d, %v", height, err)
		}
	}
}
