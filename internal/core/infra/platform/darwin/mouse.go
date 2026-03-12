//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "accessibility.h"
*/
import "C"

import (
	"image"
	"sync"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

var (
	mouseDown    bool
	mouseDownPos image.Point
	mouseDownMu  sync.RWMutex
)

// SetLeftMouseDown sets the left mouse button down state.
func SetLeftMouseDown(down bool, pos image.Point) {
	mouseDownMu.Lock()
	defer mouseDownMu.Unlock()
	mouseDown = down
	mouseDownPos = pos
}

// IsLeftMouseDown returns whether the left mouse button is down.
func IsLeftMouseDown() bool {
	mouseDownMu.RLock()
	defer mouseDownMu.RUnlock()

	return mouseDown
}

// GetLastMouseDownPosition returns the last position where mouse down occurred.
func GetLastMouseDownPosition() image.Point {
	mouseDownMu.RLock()
	defer mouseDownMu.RUnlock()

	return mouseDownPos
}

// ClearLeftMouseDownState clears the left mouse button down state.
func ClearLeftMouseDownState() {
	mouseDownMu.Lock()
	defer mouseDownMu.Unlock()
	mouseDown = false
	mouseDownPos = image.Point{}
}

// EnsureMouseUp ensures that if the left mouse button is down, it is released.
func EnsureMouseUp() {
	if IsLeftMouseDown() {
		_ = LeftMouseUp()
		ClearLeftMouseDownState()
	}
}

// MoveMouse moves the mouse cursor to the specified point.
// If bypassSmooth is true, smooth cursor configuration is bypassed.
func MoveMouse(point image.Point, bypassSmooth bool) {
	var eventType C.CGEventType = C.kCGEventMouseMoved
	if IsLeftMouseDown() {
		eventType = C.kCGEventLeftMouseDragged
	}

	cfg := config.Global()
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && !bypassSmooth {
		MoveMouseSmooth(point, cfg.SmoothCursor.Steps, cfg.SmoothCursor.Delay, uint32(eventType))
	} else {
		pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
		C.moveMouseWithType(pos, eventType)
	}
}

// MoveMouseSmooth moves the mouse cursor smoothly to the specified point.
func MoveMouseSmooth(end image.Point, steps, delay int, eventType uint32) {
	current := CursorPosition()
	C.moveMouseSmoothWithType(
		C.CGPoint{x: C.double(current.X), y: C.double(current.Y)},
		C.CGPoint{x: C.double(end.X), y: C.double(end.Y)},
		C.int(steps),
		C.int(delay),
		C.CGEventType(eventType),
	)
}

// CursorPosition returns the current cursor position.
func CursorPosition() image.Point {
	pos := C.getCurrentCursorPosition()

	return image.Point{X: int(pos.x), Y: int(pos.y)}
}

// LeftClickAtPoint performs a left mouse click at the specified point.
func LeftClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// RightClickAtPoint performs a right mouse click at the specified point.
func RightClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performRightClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform right-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// MiddleClickAtPoint performs a middle mouse click at the specified point.
func MiddleClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performMiddleClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform middle-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// LeftMouseDownAtPoint performs a left mouse down action at the specified point.
func LeftMouseDownAtPoint(point image.Point) error {
	SetLeftMouseDown(true, point)

	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftMouseDownAtPosition(pos)
	if result == 0 {
		ClearLeftMouseDownState()

		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-mouse-down at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// LeftMouseUpAtPoint performs a left mouse up action at the specified point.
func LeftMouseUpAtPoint(point image.Point) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftMouseUpAtPosition(pos)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-mouse-up at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	ClearLeftMouseDownState()

	return nil
}

// LeftMouseUp performs a left mouse up action at the current cursor position.
func LeftMouseUp() error {
	result := C.performLeftMouseUpAtCursor()
	if result == 0 {
		return derrors.New(derrors.CodeActionFailed, "failed to perform left-mouse-up at cursor")
	}

	ClearLeftMouseDownState()

	return nil
}

// ScrollAtCursor performs a scroll action at the current cursor position.
func ScrollAtCursor(deltaX, deltaY int) error {
	result := C.scrollAtCursor(C.int(deltaX), C.int(deltaY))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to scroll at cursor with delta (%d, %d)",
			deltaX,
			deltaY,
		)
	}

	return nil
}
