//go:build linux

package linux

import (
	"image"
	"sync/atomic"
)

// No Linux display server lets a client leave one window out of a capture, so
// the overlay takes itself off around one instead. Its registered hider clears
// what it shows inside the captured rectangle, waits until the captured screen
// no longer has it, runs the capture, and puts back exactly what it showed. The
// user sees that part of the overlay vanish for at least one frame.
//
// Each overlay backend decides what to wait for. Wayland waits for the frame
// callback for the cleared buffer, which covers wlr-screencopy and a portal
// stream alike. X11 waits for the server's sync, then for a compositor repaint.
var ownWindowsHider atomic.Pointer[func(region image.Rectangle, capture func() error) error]

// SetOwnWindowsHider registers the function that takes Neru's overlay off the
// screen around a capture, or clears it with nil. hider runs capture with the
// overlay cleared inside region, the global rectangle captured, or all of it
// when region is empty. It puts back what the overlay showed, and returns
// capture's error, or an error of its own when it could not clear the overlay.
func SetOwnWindowsHider(hider func(region image.Rectangle, capture func() error) error) {
	if hider == nil {
		ownWindowsHider.Store(nil)

		return
	}

	ownWindowsHider.Store(&hider)
}

// ownWindowsDrawnOn answers where Neru's overlay drew, in global pixels.
var ownWindowsDrawnOn atomic.Pointer[func() []image.Rectangle]

// SetOwnWindowsDrawnOn registers the function that answers where Neru's
// overlay has drawn since it was last cleared, in global pixels, or clears it
// with nil. The rectangles may cover more than was drawn, never less.
func SetOwnWindowsDrawnOn(drawnOn func() []image.Rectangle) {
	if drawnOn == nil {
		ownWindowsDrawnOn.Store(nil)

		return
	}

	ownWindowsDrawnOn.Store(&drawnOn)
}

// OwnWindowsDrawnOn returns where Neru's overlay has drawn, so a caller
// comparing captures taken with it left up (CaptureScreenRegionWithOverlay)
// can leave those pixels out. It is nil with no overlay registered.
func OwnWindowsDrawnOn() []image.Rectangle {
	drawnOn := ownWindowsDrawnOn.Load()
	if drawnOn == nil {
		return nil
	}

	return (*drawnOn)()
}

// CaptureExcludesOwnWindows reports whether CaptureScreenRegion leaves Neru's
// overlay out, which takes an overlay that registered a hider.
func CaptureExcludesOwnWindows() bool {
	return registeredHider() != nil
}

func registeredHider() func(region image.Rectangle, capture func() error) error {
	hider := ownWindowsHider.Load()
	if hider == nil {
		return nil
	}

	return *hider
}

// withOwnWindowsHidden runs capture, of region in global pixels, with the
// overlay taken off that part of the screen, when hide is set and an overlay
// registered a hider, and plainly otherwise.
func withOwnWindowsHidden(
	hide bool,
	region image.Rectangle,
	capture func() (*image.RGBA, error),
) (*image.RGBA, error) {
	hider := registeredHider()
	if !hide || hider == nil {
		return capture()
	}

	var img *image.RGBA

	err := hider(region, func() error {
		var captureErr error

		img, captureErr = capture()

		return captureErr
	})
	if err != nil {
		return nil, err
	}

	return img, nil
}
