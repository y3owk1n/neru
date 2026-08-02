//go:build darwin

package native

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

func platformActiveScreenBounds() image.Rectangle {
	return darwin.ActiveScreenBounds()
}
