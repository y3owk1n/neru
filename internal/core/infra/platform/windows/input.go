//go:build windows

// internal/core/infra/platform/windows/input.go
// Mouse and keyboard synthesis via SendInput.
// Does not implement accessibility element actions.

package windows

import (
	"errors"
	"fmt"
	"image"
	"unsafe"
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseeventfMove       = 0x0001
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfWheel      = 0x0800
	mouseeventfAbsolute   = 0x8000

	wheelDelta = 120
)

// mouseInput and input mirror Win32 MOUSEINPUT/INPUT on 64-bit Windows (40 bytes).
// SendInput rejects the wrong size with ERROR_INVALID_PARAMETER.
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	_           uint32
	dwExtraInfo uintptr
}

type input struct {
	inputType uint32
	_         uint32
	mi        mouseInput
}

// Compile-time guard: INPUT must be 40 bytes on 64-bit Windows targets.
var _ [40 - unsafe.Sizeof(input{})]byte

var procSendInput = user32.NewProc("SendInput")

var errSendInputFailed = errors.New("SendInput failed")

func sendMouseInput(flags uint32, data uint32) error {
	var event input

	event.inputType = inputMouse
	event.mi.dwFlags = flags
	event.mi.mouseData = data

	ret, _, err := procSendInput.Call(
		1,
		uintptr(unsafe.Pointer(&event)),
		unsafe.Sizeof(event),
	)
	if ret == 0 {
		if err != nil {
			return fmt.Errorf("SendInput: %w", err)
		}

		return errSendInputFailed
	}

	return nil
}

// MoveMouseTo moves the cursor to the given screen point.
func MoveMouseTo(point image.Point) error {
	return moveCursorTo(point)
}

// LeftClickAt performs a left click at the given point.
func LeftClickAt(point image.Point) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	err = sendMouseInput(mouseeventfLeftDown, 0)
	if err != nil {
		return err
	}

	return sendMouseInput(mouseeventfLeftUp, 0)
}

// RightClickAt performs a right click at the given point.
func RightClickAt(point image.Point) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	err = sendMouseInput(mouseeventfRightDown, 0)
	if err != nil {
		return err
	}

	return sendMouseInput(mouseeventfRightUp, 0)
}

// MiddleClickAt performs a middle click at the given point.
func MiddleClickAt(point image.Point) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	err = sendMouseInput(mouseeventfMiddleDown, 0)
	if err != nil {
		return err
	}

	return sendMouseInput(mouseeventfMiddleUp, 0)
}

// LeftMouseDown presses the left button at the given point.
func LeftMouseDown(point image.Point) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	return sendMouseInput(mouseeventfLeftDown, 0)
}

// LeftMouseUp releases the left button at the given point.
func LeftMouseUp(point image.Point) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	return sendMouseInput(mouseeventfLeftUp, 0)
}

// ScrollWheel scrolls vertically at the current cursor position.
func ScrollWheel(deltaLines int) error {
	if deltaLines == 0 {
		return nil
	}

	return sendMouseInput(mouseeventfWheel, uint32(int32(deltaLines)*wheelDelta))
}

// CurrentCursorPosition returns the current cursor location.
func CurrentCursorPosition() (image.Point, error) {
	return cursorPosition()
}
