//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"unsafe"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// Mouse and keyboard synthesis via SendInput.
// Does not implement accessibility element actions.
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

	keyeventfKeyUp = 0x0002

	// neruInjectedTag rides in dwExtraInfo on every keyboard event this
	// process synthesizes, so the low-level keyboard hook can tell Neru's own
	// injection apart from a real keypress. Without it a modified scroll's
	// ctrl comes straight back through the hook and is read as the user
	// tapping ctrl. Deliberately narrower than filtering LLKHF_INJECTED, which
	// would also hide injection by other tools.
	neruInjectedTag = 0x4E455255 // 'N','E','R','U'

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

// keyboardInput and keyInput mirror Win32 KEYBDINPUT/INPUT. KEYBDINPUT is the
// smaller arm of the INPUT union, so the trailing padding is what keeps the
// struct at the 40 bytes SendInput's cbSize demands.
type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type keyInput struct {
	inputType uint32
	_         uint32
	ki        keyboardInput
	_         [8]byte
}

// Compile-time guards: INPUT must be 40 bytes on 64-bit Windows targets,
// whichever arm of the union is in use.
var (
	_ [40 - unsafe.Sizeof(input{})]byte
	_ [40 - unsafe.Sizeof(keyInput{})]byte
)

var procSendInput = user32.NewProc("SendInput")

var errSendInputFailed = errors.New("SendInput failed")

// sendOneInput posts a single already-filled INPUT record. Both union arms go
// through here so the call convention and the failure reporting are stated once.
func sendOneInput(event unsafe.Pointer, size uintptr) error {
	ret, _, err := procSendInput.Call(1, uintptr(event), size)
	if ret == 0 {
		if err != nil {
			return fmt.Errorf("SendInput: %w", err)
		}

		return errSendInputFailed
	}

	return nil
}

func sendMouseInput(flags uint32, data uint32) error {
	var event input

	event.inputType = inputMouse
	event.mi.dwFlags = flags
	event.mi.mouseData = data

	return sendOneInput(unsafe.Pointer(&event), unsafe.Sizeof(event))
}

// sendKeyboardInput presses or releases one virtual key.
func sendKeyboardInput(virtualKey uint16, isUp bool) error {
	var event keyInput

	event.inputType = inputKeyboard
	event.ki.wVk = virtualKey
	event.ki.dwExtraInfo = neruInjectedTag

	if isUp {
		event.ki.dwFlags = keyeventfKeyUp
	}

	return sendOneInput(unsafe.Pointer(&event), unsafe.Sizeof(event))
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

// buttonFlags holds the SendInput flags that press and release one mouse button.
type buttonFlags struct {
	down uint32
	up   uint32
}

// flagsForButton returns the SendInput flags addressing the given button.
func flagsForButton(button action.MouseButton) buttonFlags {
	switch button {
	case action.ButtonRight:
		return buttonFlags{down: mouseeventfRightDown, up: mouseeventfRightUp}
	case action.ButtonMiddle:
		return buttonFlags{down: mouseeventfMiddleDown, up: mouseeventfMiddleUp}
	case action.ButtonLeft:
		fallthrough
	default:
		return buttonFlags{down: mouseeventfLeftDown, up: mouseeventfLeftUp}
	}
}

// MouseDown presses the given button at the given point.
func MouseDown(point image.Point, button action.MouseButton) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	return sendMouseInput(flagsForButton(button).down, 0)
}

// MouseUp releases the given button at the given point.
func MouseUp(point image.Point, button action.MouseButton) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	return sendMouseInput(flagsForButton(button).up, 0)
}

// ScrollWheel scrolls vertically at the current cursor position, holding
// modifiers down for the duration.
//
// A SendInput wheel event carries no modifier field — unlike a CGEvent, which
// takes flags — so the only way to present a held ctrl is to press the real
// key, wheel, and release it. Releasing only what this call pressed leaves a
// modifier the user is physically holding untouched.
func ScrollWheel(deltaLines int, modifiers action.Modifiers) error {
	if deltaLines == 0 {
		return nil
	}

	pressErr := pressModifiers(modifiers)
	if pressErr != nil {
		return pressErr
	}

	defer releaseModifiers(modifiers)

	return sendMouseInput(mouseeventfWheel, uint32(int32(deltaLines)*wheelDelta))
}

// modifierKeys lists the virtual-key code (keys.go) for each modifier bit, in
// the order they are pressed. Release walks it backwards.
var modifierKeys = []struct {
	bit action.Modifiers
	key uint16
}{
	{bit: action.ModShift, key: vkShift},
	{bit: action.ModCtrl, key: vkControl},
	{bit: action.ModAlt, key: vkMenu},
	{bit: action.ModCmd, key: vkLWin},
}

// pressModifiers holds down every key in modifiers, releasing what it already
// pressed if one of them fails so a partial set is never left latched.
//
// SendInput delivers these to Neru's own low-level keyboard hook, which runs
// its callback inline on the hook thread and from there reaches the mode
// handler's lock. neruInjectedTag is what stops that: keep it on every
// synthesized key, or this becomes a call into the handler from whatever
// goroutine is injecting.
func pressModifiers(modifiers action.Modifiers) error {
	var pressed action.Modifiers

	for _, modifier := range modifierKeys {
		if !modifiers.Has(modifier.bit) {
			continue
		}

		err := sendKeyboardInput(modifier.key, false)
		if err != nil {
			releaseModifiers(pressed)

			return err
		}

		pressed |= modifier.bit
	}

	return nil
}

// releaseModifiers lets go of every key in modifiers, in reverse press order.
// Errors are dropped: a release that fails has nothing better to try, and
// reporting it would mask the outcome of the action it wraps.
func releaseModifiers(modifiers action.Modifiers) {
	for _, modifier := range slices.Backward(modifierKeys) {
		if modifiers.Has(modifier.bit) {
			_ = sendKeyboardInput(modifier.key, true)
		}
	}
}

// CurrentCursorPosition returns the current cursor location.
func CurrentCursorPosition() (image.Point, error) {
	return cursorPosition()
}
