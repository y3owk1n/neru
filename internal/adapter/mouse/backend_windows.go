//go:build windows

package mouse

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

func platformBackend() backend {
	return backend{
		isDown:     windows.IsMouseButtonDown,
		releaseAll: windows.EnsureMouseUp,
		click:      windows.ClickAt,
		down:       windows.MouseDown,
		up:         windows.MouseUp,
		move:       func(point image.Point) { _ = windows.MoveMouseTo(point) },
		scroll:     windows.ScrollWheel,
	}
}
