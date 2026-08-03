//go:build darwin

package darwin

/*
#include "accessibility.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"math"
	"unsafe"
)

// ActiveScreenBounds returns the active screen bounds (the screen containing the cursor).
func ActiveScreenBounds() image.Rectangle {
	rect := C.NeruGetActiveScreenBounds()

	return image.Rect(
		int(rect.origin.x),
		int(rect.origin.y),
		int(rect.origin.x+rect.size.width),
		int(rect.origin.y+rect.size.height),
	)
}

// ScreenNames returns the localized display names of all connected screens.
// The C side returns a NUL-separated buffer (each name terminated by '\0')
// so that display names containing commas are handled correctly.
func ScreenNames() []string {
	var bufLen C.int

	cNames := C.NeruGetScreenNames(&bufLen)
	if cNames == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cNames)) //nolint:nlreturn

	totalLen := int(bufLen)
	if totalLen == 0 {
		return nil
	}

	// Walk the NUL-separated buffer using the known length as the bound.
	var names []string

	offset := 0
	for offset < totalLen {
		name := C.GoString((*C.char)(unsafe.Add(unsafe.Pointer(cNames), offset)))
		if len(name) == 0 {
			// Skip empty names (e.g. a hypothetical empty localizedName)
			// and advance past the lone NUL terminator.
			offset++

			continue
		}

		names = append(names, name)
		offset += len(name) + 1
	}

	return names
}

// ScreenBoundsByName returns the screen bounds for the display with the given
// localized name (case-insensitive). The second return value is false when no
// screen matches.
func ScreenBoundsByName(name string) (image.Rectangle, bool) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName)) //nolint:nlreturn

	var found C.int

	rect := C.NeruGetScreenBoundsByName(cName, &found)
	if found == 0 {
		return image.Rectangle{}, false
	}

	return image.Rect(
		int(rect.origin.x),
		int(rect.origin.y),
		int(rect.origin.x+rect.size.width),
		int(rect.origin.y+rect.size.height),
	), true
}

// IsMissionControlActive returns true if Mission Control is active.
func IsMissionControlActive() bool {
	return bool(C.NeruIsMissionControlActive())
}

// IsScreenZoomed reports whether the macOS Accessibility Zoom feature is
// currently zoomed in. Cursor positioning does not depend on this — synthetic
// mouse events are posted at the session tap so they bypass the window server's
// zoom coordinate transform — but it is useful when diagnosing pointer bugs.
func IsScreenZoomed() bool {
	return bool(C.NeruIsScreenZoomed())
}

// ZoomViewportAt returns the region that Accessibility Zoom currently shows on
// the display containing point, in global CG coordinates. The viewport edges are
// fractional, so this is the smallest integer rectangle that fully contains it.
// The second return value is false when that display is not magnified — zoom is
// off, zoomed all the way out, or magnifying a different display — or when the
// SkyLight zoom SPI that backs it is unavailable.
func ZoomViewportAt(point image.Point) (image.Rectangle, bool) {
	var viewport C.CGRect

	target := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	if C.NeruGetZoomViewportForPoint(target, &viewport) == 0 {
		return image.Rectangle{}, false
	}

	return image.Rect(
		int(math.Floor(float64(viewport.origin.x))),
		int(math.Floor(float64(viewport.origin.y))),
		int(math.Ceil(float64(viewport.origin.x+viewport.size.width))),
		int(math.Ceil(float64(viewport.origin.y+viewport.size.height))),
	), true
}

// FocusedWindowBounds returns the bounds of the focused (frontmost) window.
// Returns the bounds and true if a window was found, or a zero rectangle and
// false if no focused window exists (e.g. the desktop is focused).
func FocusedWindowBounds() (image.Rectangle, bool) {
	rect := C.NeruGetFocusedWindowFrame()

	// CGRectZero means no window was found.
	if rect.size.width == 0 && rect.size.height == 0 {
		return image.Rectangle{}, false
	}

	return image.Rect(
		int(rect.origin.x),
		int(rect.origin.y),
		int(rect.origin.x+rect.size.width),
		int(rect.origin.y+rect.size.height),
	), true
}
