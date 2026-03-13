//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "accessibility.h"
*/
import "C"

import (
	"image"
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

// IsMissionControlActive returns true if Mission Control is active.
func IsMissionControlActive() bool {
	return bool(C.isMissionControlActive())
}
