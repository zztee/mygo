//go:build darwin

package darwin

import (
	"bytes"
	"image/png"
	"math"
	"testing"
)

func TestSnapshotWidth(t *testing.T) {
	for _, tt := range []struct {
		pixels      int
		scale, want float64
	}{
		{640, 2, 320}, {1280, 2, 640}, {1920, 2, 960}, {2560, 2, 1280}, {641, 2, 320.5}, {640, 1.5, 640 / 1.5},
		{640, 0, 640}, {640, math.NaN(), 640}, {640, math.Inf(1), 640},
	} {
		if got := snapshotWidth(tt.pixels, tt.scale); got != tt.want {
			t.Errorf("%d / %v=%v, want %v", tt.pixels, tt.scale, got, tt.want)
		}
	}
}

func TestSnapshotConfigurationNative(t *testing.T) {
	load()
	withPool(func() {
		config := send(send(class("WKSnapshotConfiguration"), "alloc"), "init")
		defer release(config)
		rect := NSRect{Origin: NSPoint{X: 7, Y: 19}, Size: NSSize{Width: 640, Height: 4000}}
		msgSetRect(config, sel("setRect:"), rect)
		if got := msgRect(config, sel("rect")); got != rect {
			t.Fatalf("rect=%+v", got)
		}
		send(config, "setSnapshotWidth:", uintptr(msgFloatID(class("NSNumber"), sel("numberWithDouble:"), 320.5)))
		if got := msgFloat(send(config, "snapshotWidth"), sel("doubleValue")); got != 320.5 {
			t.Fatalf("width=%v", got)
		}
	})
}

func TestSnapshotFractionalImagePixels(t *testing.T) {
	load()
	withPool(func() {
		rep := msgInitBitmap(send(class("NSBitmapImageRep"), "alloc"), sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bytesPerRow:bitsPerPixel:"), 0, 301, 201, 8, 4, true, false, nsString("NSCalibratedRGBColorSpace"), 0, 0)
		if rep == 0 {
			t.Fatal("bitmap allocation failed")
		}
		defer release(rep)
		logical := NSSize{Width: 150.5, Height: 100.5}
		msgSetSize(rep, sel("setSize:"), logical)
		img := send(send(class("NSImage"), "alloc"), "init")
		defer release(img)
		send(img, "addRepresentation:", uintptr(rep))
		msgSetSize(img, sel("setSize:"), logical)
		data := pngFromSnapshot(img)
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != 301 || cfg.Height != 201 {
			t.Fatalf("bitmap=%dx%d, %v", cfg.Width, cfg.Height, err)
		}
	})
}

// TestSnapshotConcurrentResize uses AppKit's actual autoresizing behavior,
// including a parent resize while the child has its temporary tall frame.
func TestSnapshotConcurrentResize(t *testing.T) {
	load()
	withPool(func() {
		original := NSRect{Size: NSSize{Width: 640, Height: 480}}
		expanded := NSRect{Size: NSSize{Width: 640, Height: 3600}}
		parent := msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), original)
		defer release(parent)
		child := msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), original)
		defer release(child)
		send(child, "setAutoresizingMask:", nsViewWidthHeightSizable)
		send(parent, "addSubview:", uintptr(child))
		msgSetRect(child, sel("setFrame:"), expanded)
		resized := NSRect{Size: NSSize{Width: 740, Height: 580}}
		msgSetRect(parent, sel("setFrame:"), resized)
		current := msgRect(child, sel("frame"))
		if current.Size.Height != 3700 {
			t.Fatalf("AppKit did not resize expanded child: %v", current)
		}
		restored := restoredSnapshotFrame(original, expanded, current)
		msgSetRect(child, sel("setFrame:"), restored)
		if got := msgRect(child, sel("frame")); got != resized {
			t.Fatalf("restored=%v, want %v", got, resized)
		}
	})
}
