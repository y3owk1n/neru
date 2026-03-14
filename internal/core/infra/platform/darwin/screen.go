//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "accessibility.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"strings"
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
func ScreenNames() []string {
	cNames := C.getScreenNames()
	if cNames == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cNames)) //nolint:nlreturn

	goNames := C.GoString(cNames)
	if goNames == "" {
		return nil
	}

	return strings.Split(goNames, ",")
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
