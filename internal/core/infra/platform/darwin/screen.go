//go:build darwin

package darwin

/*
#include "accessibility.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"unsafe"
)

// ActiveScreenBounds returns the active screen bounds (the screen containing the cursor).
func ActiveScreenBounds() image.Rectangle {
	rect := C.getActiveScreenBounds()

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

	cNames := C.getScreenNames(&bufLen)
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

	rect := C.getScreenBoundsByName(cName, &found)
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
	return bool(C.isMissionControlActive())
}
