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
func MoveMouse(point image.Point, bypassSmooth bool) {
	C.moveMouseWithType(C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}, C.kCGEventMouseMoved)
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
	C.performLeftClickAtPosition(
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		C.bool(restoreCursor),
	)

	return nil
}

// RightClickAtPoint performs a right mouse click at the specified point.
func RightClickAtPoint(point image.Point, restoreCursor bool) error {
	C.performRightClickAtPosition(
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		C.bool(restoreCursor),
	)

	return nil
}

// MiddleClickAtPoint performs a middle mouse click at the specified point.
func MiddleClickAtPoint(point image.Point, restoreCursor bool) error {
	C.performMiddleClickAtPosition(
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		C.bool(restoreCursor),
	)

	return nil
}

// LeftMouseDownAtPoint performs a left mouse down action at the specified point.
func LeftMouseDownAtPoint(point image.Point) error {
	C.performLeftMouseDownAtPosition(C.CGPoint{x: C.double(point.X), y: C.double(point.Y)})

	return nil
}

// LeftMouseUpAtPoint performs a left mouse up action at the specified point.
func LeftMouseUpAtPoint(point image.Point) error {
	C.performLeftMouseUpAtPosition(C.CGPoint{x: C.double(point.X), y: C.double(point.Y)})

	return nil
}

// LeftMouseUp performs a left mouse up action at the current cursor position.
func LeftMouseUp() error {
	C.performLeftMouseUpAtCursor()

	return nil
}

// ScrollAtCursor performs a scroll action at the current cursor position.
func ScrollAtCursor(deltaX, deltaY int) error {
	C.scrollAtCursor(C.int(deltaX), C.int(deltaY))

	return nil
}
