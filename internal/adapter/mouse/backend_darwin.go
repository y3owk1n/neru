//go:build darwin

package mouse

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func platformBackend() backend {
	return backend{
		isDown:     darwin.IsMouseButtonDown,
		releaseAll: darwin.EnsureMouseUp,
		click: func(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
			switch button {
			case action.ButtonRight:
				return darwin.RightClickAtPoint(point, false, modifiers)
			case action.ButtonMiddle:
				return darwin.MiddleClickAtPoint(point, false, modifiers)
			case action.ButtonLeft:
			}

			return darwin.LeftClickAtPoint(point, false, modifiers)
		},
		down: darwin.MouseDownAtPoint,
		up:   darwin.MouseUpAtPoint,
		move: func(point image.Point) error {
			darwin.MoveMouse(point, false)

			return nil
		},
		scroll: darwin.ScrollAtCursor,
	}
}
