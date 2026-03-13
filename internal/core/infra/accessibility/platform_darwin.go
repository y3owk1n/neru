//go:build darwin

package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

func platformActiveScreenBounds() image.Rectangle {
	return darwin.ActiveScreenBounds()
}
