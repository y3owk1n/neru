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

	overlay := &sharedOverlay{
		srf: &changeTrackedSurface{overlaySurface: &recordingSurface{scale: 1}},
	}

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
		t.Fatal(
			"a clear not yet flushed reports nothing on screen, while the rectangle still shows",
		)
	}

	overlay.Flush()

	if !overlay.showsNothing() {
		t.Fatal("a committed empty frame reports something on screen")
	}
}

// TestSharedOverlay_DrawnOn_CoversWhatWasDrawnUntilAClear pins what the
// probes that wait for a scroll to settle leave out of their comparison. It
// has to cover every pixel drawn since the last clear, a stroke's width
// included, but not the inside of an outline, and nothing once the surface is
// cleared.
func TestSharedOverlay_DrawnOn_CoversWhatWasDrawnUntilAClear(t *testing.T) {
	t.Parallel()

	overlay := &sharedOverlay{
		srf: &changeTrackedSurface{overlaySurface: &recordingSurface{scale: 1}},
	}

	cursor := image.Rect(100, 100, 110, 120)
	container := image.Rect(50, 50, 400, 300)

	overlay.srf.rectPrim(cursor, 0xFF000000, 0, 0)
	overlay.srf.roundedRectPrim(container, 6, 0, 0xFF000000, 2)

	covered := func(want image.Rectangle) bool {
		for _, drawn := range overlay.drawnOn() {
			if want.In(drawn) {
				return true
			}
		}

		return false
	}

	edges := []image.Rectangle{
		image.Rect(container.Min.X-1, container.Min.Y-1, container.Max.X+1, container.Min.Y+1),
		image.Rect(container.Min.X-1, container.Max.Y-1, container.Max.X+1, container.Max.Y+1),
		image.Rect(container.Min.X-1, container.Min.Y-1, container.Min.X+1, container.Max.Y+1),
		image.Rect(container.Max.X-1, container.Min.Y-1, container.Max.X+1, container.Max.Y+1),
	}

	if !covered(cursor) {
		t.Fatalf("drawnOn() = %v, want it to cover the filled %v", overlay.drawnOn(), cursor)
	}

	for _, edge := range edges {
		if !covered(edge) {
			t.Fatalf(
				"drawnOn() = %v, want it to cover the outline's edge %v",
				overlay.drawnOn(),
				edge,
			)
		}
	}

	// The page inside an outline is what a probe watches, so it stays in.
	if covered(image.Rect(200, 150, 201, 151)) {
		t.Fatalf("drawnOn() = %v covers the inside of the outline %v", overlay.drawnOn(), container)
	}

	overlay.Clear()

	if drawn := overlay.drawnOn(); len(drawn) != 0 {
		t.Fatalf("drawnOn() after a clear = %v, want nothing", drawn)
	}
}
