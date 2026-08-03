//go:build darwin

package darwin

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// PlatformActiveScreenBounds returns the active screen in global coordinates.
func PlatformActiveScreenBounds() image.Rectangle {
	return darwin.ActiveScreenBounds()
}
