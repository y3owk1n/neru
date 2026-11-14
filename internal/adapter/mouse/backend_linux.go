//go:build linux

package mouse

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func platformBackend() backend {
	return backend{
		isDown:     linux.IsMouseButtonDown,
		releaseAll: linux.EnsureMouseUp,
		click: func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
			switch button {
			case action.ButtonRight:
				return linux.RightClickAtPoint(point, false, modifiers)
			case action.ButtonMiddle:
				return linux.MiddleClickAtPoint(point, false, modifiers)
			case action.ButtonLeft:
			}

			return linux.LeftClickAtPoint(point, false, modifiers)
		},
		down:   linux.MouseDownAtPoint,
		up:     linux.MouseUpAtPoint,
		move:   func(point image.Point) { linux.MoveMouseToPoint(point, false) },
		scroll: linux.ScrollAtCursor,
	}
}
