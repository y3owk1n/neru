//go:build windows

package mouse

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

func platformBackend() backend {
	return backend{
		isDown:     windows.IsMouseButtonDown,
		releaseAll: windows.EnsureMouseUp,
		click:      windows.ClickAt,
		down:       windows.MouseDown,
		up:         windows.MouseUp,
		move:       windows.MoveMouseTo,
		scroll:     windows.ScrollWheel,
	}
}
