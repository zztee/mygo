//go:build darwin

package darwin

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/platform"
)

// snapshotWidth converts output pixels to the logical width WebKit expects.
func snapshotWidth(pixels int, scale float64) float64 {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}
	return float64(pixels) / scale
}

// pageCaptures guards the temporary view frame against overlapping captures.
// It is only accessed on the main thread and allocated on first use.
var pageCaptures map[*window]bool

// CapturePageWithOptions captures an opt-in region without adding a method
// to window's exported method set, which reflected services would retain.
func CapturePageWithOptions(n platform.Window, o platform.CapturePageOptions, cb func([]byte, error)) {
	w, ok := n.(*window)
	if !ok {
		n.CapturePage(cb)
		return
	}
	if pageCaptures[w] {
		cb(nil, errors.New("mygo: a page capture is already in progress"))
		return
	}
	if pageCaptures == nil {
		pageCaptures = make(map[*window]bool)
	}
	pageCaptures[w] = true
	web := retain(w.web)
	finish := func(data []byte, err error) {
		delete(pageCaptures, w)
		release(web)
		cb(data, err)
	}
	// Resizing the layout viewport can clamp scroll offsets. Read them first
	// and retain the view until both the snapshot and restoration complete.
	w.CallAsyncFunction("return JSON.stringify([window.scrollX, window.scrollY]);", func(value string, err error) {
		if w.closed {
			finish(nil, errDestroyed)
			return
		}
		var scroll [2]float64
		if err == nil {
			err = json.Unmarshal([]byte(value), &scroll)
		}
		if err != nil {
			finish(nil, err)
			return
		}
		withPool(func() {
			// Expand only the web view, avoiding AppKit's window-size limit.
			frame := msgRect(web, sel("frame"))
			captureFrame := frame
			if r := o.Rect; r != (platform.Rect{}) {
				captureFrame.Size.Width = math.Max(frame.Size.Width, float64(r.X)+float64(r.Width))
				captureFrame.Size.Height = math.Max(frame.Size.Height, float64(r.Y)+float64(r.Height))
			}
			msgSetRect(web, sel("setFrame:"), captureFrame)
			config := send(send(class("WKSnapshotConfiguration"), "alloc"), "init")
			defer release(config)
			if r := o.Rect; r != (platform.Rect{}) {
				msgSetRect(config, sel("setRect:"), NSRect{
					Origin: NSPoint{X: float64(r.X), Y: float64(r.Y)},
					Size:   NSSize{Width: float64(r.Width), Height: float64(r.Height)},
				})
			}
			if o.Width > 0 {
				width := snapshotWidth(o.Width, msgFloat(w.win, sel("backingScaleFactor")))
				send(config, "setSnapshotWidth:", uintptr(msgFloatID(class("NSNumber"), sel("numberWithDouble:"), width)))
			}
			blk := newBlock(func(_ objc.Block, img id, nativeErr id) {
				if w.closed {
					finish(nil, errDestroyed)
					return
				}
				// Autoresizing adds the parent's delta to our temporary dimensions.
				// Remove only our expansion, keeping any intervening window resize.
				current := msgRect(web, sel("frame"))
				msgSetRect(web, sel("setFrame:"), restoredSnapshotFrame(frame, captureFrame, current))
				var data []byte
				var captureErr error
				if nativeErr != 0 {
					captureErr = nsError(nativeErr)
				} else if img == 0 {
					captureErr = errors.New("mygo: WebKit returned no page snapshot")
				} else {
					withPool(func() { data = pngFromSnapshot(img) })
					if len(data) == 0 {
						captureErr = errors.New("mygo: could not encode page snapshot")
					}
				}
				// WebKit orders this async call after the viewport update. Do not
				// await an animation frame: hidden views may never get one.
				// Bypass CSS smooth scrolling so callers see the offset immediately.
				restore := fmt.Sprintf(`window.scrollTo({left:%g, top:%g, behavior:"instant"}); return "";`, scroll[0], scroll[1])
				w.CallAsyncFunction(restore, func(_ string, restoreErr error) {
					if captureErr == nil {
						captureErr = restoreErr
					}
					finish(data, captureErr)
				})
			})
			defer blk.Release()
			send(web, "takeSnapshotWithConfiguration:completionHandler:", uintptr(config), uintptr(blk))
		})
	})
}

func restoredSnapshotFrame(original, expanded, current NSRect) NSRect {
	current.Size.Width = math.Max(0, current.Size.Width-(expanded.Size.Width-original.Size.Width))
	current.Size.Height = math.Max(0, current.Size.Height-(expanded.Size.Height-original.Size.Height))
	return current
}

// pngFromSnapshot draws the snapshot into a bitmap with its original pixel
// dimensions. NSImage's automatic TIFF/CGImage conversion rounds fractional
// logical dimensions down before applying the display scale.
func pngFromSnapshot(img id) []byte {
	reps := send(img, "representations")
	var width, height int
	for i := 0; i < sendInt(reps, "count"); i++ {
		rep := send(reps, "objectAtIndex:", uintptr(i))
		if w := sendInt(rep, "pixelsWide"); w > width {
			width, height = w, sendInt(rep, "pixelsHigh")
		}
	}
	if width <= 0 || height <= 0 {
		return nil
	}
	rep := autorelease(msgInitBitmap(send(class("NSBitmapImageRep"), "alloc"),
		sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bytesPerRow:bitsPerPixel:"),
		0, width, height, 8, 4, true, false, nsString("NSCalibratedRGBColorSpace"), 0, 0))
	if rep == 0 {
		return nil
	}
	rep = send(rep, "bitmapImageRepByRetaggingWithColorSpace:", uintptr(send(class("NSColorSpace"), "sRGBColorSpace")))
	ctx := send(class("NSGraphicsContext"), "graphicsContextWithBitmapImageRep:", uintptr(rep))
	if ctx == 0 {
		return nil
	}
	gc := class("NSGraphicsContext")
	send(gc, "saveGraphicsState")
	send(gc, "setCurrentContext:", uintptr(ctx))
	send(ctx, "setImageInterpolation:", 1) // NSImageInterpolationNone
	const copyPixels = 1                   // NSCompositingOperationCopy
	dst := NSRect{Size: NSSize{Width: float64(width), Height: float64(height)}}
	msgDrawImage(img, sel("drawInRect:fromRect:operation:fraction:"), dst, NSRect{}, copyPixels, 1)
	send(gc, "restoreGraphicsState")
	return goBytes(send(rep, "representationUsingType:properties:", 4, uintptr(send(class("NSDictionary"), "dictionary"))))
}
