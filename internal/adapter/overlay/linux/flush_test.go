//go:build linux && cgo

package linux

import (
	"image"
	"testing"
)

// TestLinuxOverlayManager_Flush_CommitsOnlyAChange pins that a flush with
// nothing new drawn commits nothing. A mode's indicator tick flushes sixty
// times a second, and recommitting the whole surface each time kept the
// compositor repainting a full-screen overlay with nothing on it changing.
func TestLinuxOverlayManager_Flush_CommitsOnlyAChange(t *testing.T) {
	t.Parallel()

	surface := &recordingSurface{scale: 1}
	overlay := &x11Overlay{}
	overlay.srf = &changeTrackedSurface{overlaySurface: surface}
	overlayManager := &Manager{backend: linuxOverlayBackendX11, x11: overlay}

	overlayManager.Flush()

	if surface.flushes != 0 {
		t.Fatalf("flushed %d times with nothing drawn, want none", surface.flushes)
	}

	overlay.ClearRect(image.Rect(0, 0, 10, 10))
	overlayManager.Flush()
	overlayManager.Flush()

	if surface.flushes != 1 {
		t.Fatalf("flushed %d times after one change, want once", surface.flushes)
	}
}

// TestSharedOverlay_ShowsNothing_OnlyOnceAnEmptyFrameIsCommitted pins when a
// capture may skip taking the overlay off the screen. That is only while the
// frame on screen is empty. A clear that has not been flushed still leaves the
// last frame showing.
func TestSharedOverlay_ShowsNothing_OnlyOnceAnEmptyFrameIsCommitted(t *testing.T) {
	t.Parallel()

	overlay := &sharedOverlay{srf: &changeTrackedSurface{overlaySurface: &recordingSurface{scale: 1}}}

	if !overlay.showsNothing() {
		t.Fatal("a surface never drawn on reports something on screen")
	}

	overlay.srf.rectPrim(image.Rect(0, 0, 10, 10), 0xFF000000, 0, 0)
	overlay.Flush()

	if overlay.showsNothing() {
		t.Fatal("a committed rectangle reports nothing on screen")
	}

	overlay.Clear()

	if overlay.showsNothing() {
		t.Fatal("a clear not yet flushed reports nothing on screen, while the rectangle still shows")
	}

	overlay.Flush()

	if !overlay.showsNothing() {
		t.Fatal("a committed empty frame reports something on screen")
	}
}
