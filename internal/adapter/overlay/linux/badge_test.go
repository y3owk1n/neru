//go:build linux && cgo

package linux

import "testing"

// TestLinuxOverlayManager_Clear_LeavesTheIndicatorBadgesUp pins that an
// indicator badge is not on the surface a mode clears. It used to be painted
// there, so every clear blinked it off until the next indicator tick, and
// erasing it cleared part of what the mode drew under it, which left a line
// of holes across a bisect region as the badge followed the cursor.
func TestLinuxOverlayManager_Clear_LeavesTheIndicatorBadgesUp(t *testing.T) {
	t.Parallel()

	surface := &recordingSurface{scale: 1}
	overlay := &x11Overlay{}
	overlay.srf = &changeTrackedSurface{overlaySurface: surface}
	overlayManager := &Manager{backend: linuxOverlayBackendX11, x11: overlay}

	style := overlayBadgeStyle{fontSize: 12, paddingX: 4, paddingY: 2, borderWidth: 1}

	overlayManager.renderMu.Lock()
	overlayManager.modeIndicatorBadgeVisible = overlayManager.drawBadgeLocked(
		modeIndicatorBadgeSlot, 100, 100, "Bisect", overlayColors{}, style,
	)
	overlayManager.drawBadgeLocked(
		modeIndicatorBadgeSlot,
		140,
		120,
		"Bisect",
		overlayColors{},
		style,
	)
	overlayManager.renderMu.Unlock()

	if len(surface.clearedRects) != 0 || len(surface.rects) != 0 {
		t.Fatalf("moving the badge touched the mode surface: cleared %v, painted %d rectangles",
			surface.clearedRects, len(surface.rects))
	}

	overlayManager.Clear()

	if _, up := surface.badges[modeIndicatorBadgeSlot]; !up {
		t.Fatal("clearing the mode surface took the mode indicator badge down")
	}

	if !overlayManager.modeIndicatorBadgeVisible {
		t.Fatal("clearing the mode surface forgot the badge a capture has to take off")
	}
}
