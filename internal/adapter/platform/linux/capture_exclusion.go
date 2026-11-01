//go:build linux

package linux

import (
	"image"
	"sync/atomic"
)

// No Linux display server lets a client leave one window out of a screen
// capture, so a capture reads Neru's overlay as part of the screen. The overlay
// takes itself off around a capture instead. It registers a hider here, which
// clears what it shows, waits until the screen a capture reads no longer has
// it, runs the capture, and puts back exactly what it showed. The user's
// monitor shows the overlay missing for that long, at least one frame.
//
// Each overlay backend decides what to wait for. Wayland waits for the
// compositor's frame callback for the cleared buffer, which covers
// wlr-screencopy and a portal stream started after it alike. X11 waits for the
// server's sync, then for a compositing manager to repaint.
var ownWindowsHider atomic.Pointer[func(capture func() error) error]

// SetOwnWindowsHider registers the function that takes Neru's overlay off the
// screen around a capture, or clears it with nil. hider runs capture with the
// overlay cleared, puts back what the overlay showed, and returns capture's
// error, or an error of its own when it could not clear the overlay.
func SetOwnWindowsHider(hider func(capture func() error) error) {
	if hider == nil {
		ownWindowsHider.Store(nil)

		return
	}

	ownWindowsHider.Store(&hider)
}

// CaptureExcludesOwnWindows reports whether CaptureScreenRegion leaves Neru's
// overlay out, which takes an overlay that registered a hider.
func CaptureExcludesOwnWindows() bool {
	return registeredHider() != nil
}

func registeredHider() func(capture func() error) error {
	hider := ownWindowsHider.Load()
	if hider == nil {
		return nil
	}

	return *hider
}

// withOwnWindowsHidden runs capture with the overlay taken off the screen,
// when an overlay registered a hider, and plainly otherwise.
func withOwnWindowsHidden(capture func() (*image.RGBA, error)) (*image.RGBA, error) {
	hider := registeredHider()
	if hider == nil {
		return capture()
	}

	var img *image.RGBA

	err := hider(func() error {
		var captureErr error

		img, captureErr = capture()

		return captureErr
	})
	if err != nil {
		return nil, err
	}

	return img, nil
}
