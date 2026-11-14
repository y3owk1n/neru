//go:build linux

package mouse

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux/inject"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func platformBackend() backend {
	return backend{
		isDown:     inject.IsMouseButtonDown,
		releaseAll: inject.EnsureMouseUp,
		click: func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
			switch button {
			case action.ButtonRight:
				return inject.RightClickAtPoint(point, false, modifiers)
			case action.ButtonMiddle:
				return inject.MiddleClickAtPoint(point, false, modifiers)
			case action.ButtonLeft:
			}

			return inject.LeftClickAtPoint(point, false, modifiers)
		},
		down:   inject.MouseDownAtPoint,
		up:     inject.MouseUpAtPoint,
		move:   inject.MoveMouseToPoint,
		scroll: inject.ScrollAtCursor,
	}
}
