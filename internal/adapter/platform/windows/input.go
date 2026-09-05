//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
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
	mouseeventfHWheel     = 0x1000
	mouseeventfAbsolute   = 0x8000

	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002

	// neruInjectedTag rides in dwExtraInfo on every keyboard event this
	// process synthesizes, so the low-level keyboard hook can tell Neru's own
	// injection apart from a real keypress. Without it a modified scroll's
	// ctrl comes straight back through the hook and is read as the user
	// tapping ctrl. Deliberately narrower than filtering LLKHF_INJECTED, which
	// would also hide injection by other tools.
	neruInjectedTag = 0x4E455255 // 'N','E','R','U'

	wheelDelta = 120

	// scrollPixelsPerNotch maps the pixel delta every caller sends to wheel
	// notches. Linux uses the same figure for its X11 button clicks, so one
	// scroll_step travels the same number of notches on both; before it the
	// pixel count was sent as a notch count and one scroll_step of 50 pixels
	// was fifty notches.
	scrollPixelsPerNotch = 30
	// wheelUnitsPerPixel is that mapping in mouseData units: WHEEL_DELTA is
	// 120 per notch, so a pixel is four units and an application accumulates
	// the fraction the way it does for a high-resolution wheel.
	wheelUnitsPerPixel = wheelDelta / scrollPixelsPerNotch
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
	return sendKeyEvent(virtualKey, 0, keyUpFlag(isUp))
}

// sendKeyEvent posts one KEYBDINPUT record, tagged as Neru's own so the
// keyboard hook does not read it back as the user typing.
func sendKeyEvent(virtualKey uint16, scanCode uint16, flags uint32) error {
	var event keyInput

	event.inputType = inputKeyboard
	event.ki.wVk = virtualKey
	event.ki.wScan = scanCode
	event.ki.dwFlags = flags
	event.ki.dwExtraInfo = neruInjectedTag

	return sendOneInput(unsafe.Pointer(&event), unsafe.Sizeof(event))
}

func keyUpFlag(isUp bool) uint32 {
	if isUp {
		return keyeventfKeyUp
	}

	return 0
}

// MoveMouseTo moves the cursor to the given screen point.
func MoveMouseTo(point image.Point) error {
	return moveCursorTo(point)
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

// ClickAt presses and releases the given button at the given point, with
// exactly modifiers presented as held for both events. A SendInput button
// event carries no modifier field any more than a wheel event does, so the
// same hold scrollWheelNow takes is what keeps a hotkey chord still held while
// a hint is chosen from turning the click into a ctrl+click.
func ClickAt(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
	hold, err := holdModifiers(modifiers)
	if err != nil {
		return err
	}

	defer hold.release()

	flags := flagsForButton(button)

	err = buttonEventAt(point, flags.down)
	if err != nil {
		return err
	}

	return sendMouseInput(flags.up, 0)
}

// buttonEventAt moves the cursor to point and posts one button event there.
func buttonEventAt(point image.Point, flags uint32) error {
	err := moveCursorTo(point)
	if err != nil {
		return err
	}

	return sendMouseInput(flags, 0)
}

// MouseDown presses the given button at the given point and keeps modifiers
// presented until the matching MouseUp, so the drag in between carries them.
// A press that fails undoes its own hold: no release is coming to do it.
func MouseDown(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
	hold, err := holdModifiers(modifiers)
	if err != nil {
		return err
	}

	err = buttonEventAt(point, flagsForButton(button).down)
	if err != nil {
		hold.release()

		return err
	}

	hold.keepForRelease(button)

	return nil
}

// MouseUp releases the given button at the given point, undoing the hold its
// press kept, or presenting modifiers for the length of the release event
// when no press of this process is behind it.
func MouseUp(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
	hold, err := resumeModifierHold(button, modifiers)
	if err != nil {
		return err
	}

	defer hold.release()

	return buttonEventAt(point, flagsForButton(button).up)
}

// wheelEvent is one MOUSEEVENTF_WHEEL or MOUSEEVENTF_HWHEEL record, before
// it is posted.
type wheelEvent struct {
	flags uint32
	data  uint32
}

// wheelEvents turns a pixel scroll delta into the wheel records SendInput
// needs, one per axis that moves. Deltas follow Neru's shared convention:
// positive deltaY scrolls up and positive deltaX scrolls left, which is what
// macOS posts verbatim and what X11 maps to buttons 4 and 6. MOUSEEVENTF_WHEEL
// agrees on the vertical sign, but MOUSEEVENTF_HWHEEL reads positive as
// right, so the horizontal component is negated.
func wheelEvents(deltaX, deltaY int) []wheelEvent {
	return wheelRecords(int32(deltaY)*wheelUnitsPerPixel, int32(-deltaX)*wheelUnitsPerPixel)
}

// wheelRecords is wheelEvents below the sign convention: vertical and
// horizontal are already in WHEEL_DELTA units with Win32's signs, and a zero
// axis sends nothing.
func wheelRecords(vertical, horizontal int32) []wheelEvent {
	var events []wheelEvent

	if vertical != 0 {
		events = append(events, wheelEvent{flags: mouseeventfWheel, data: uint32(vertical)})
	}

	if horizontal != 0 {
		events = append(events, wheelEvent{flags: mouseeventfHWheel, data: uint32(horizontal)})
	}

	return events
}

// ScrollWheel scrolls at the current cursor position on both axes, presenting
// exactly modifiers as held for the duration — see holdModifiers for why a
// modifier the user is physically holding has to be suppressed rather than
// merely not pressed.
//
// With smooth_scroll.enabled the scroll is handed to the animator instead and
// arrives as a sequence of eased chunks, which is what the same setting does
// on macOS and Linux. The chunks are integer 120ths of a notch, so unlike X11
// the steps go below a wheel notch.
func ScrollWheel(deltaX, deltaY int, modifiers action.Modifiers) error {
	if deltaX == 0 && deltaY == 0 {
		return nil
	}

	cfg := currentWindowsConfig()
	if cfg != nil && cfg.SmoothScroll.Enabled {
		scrollAnim.animate(
			deltaX,
			deltaY,
			modifiers,
			cfg.SmoothScroll.Steps,
			cfg.SmoothScroll.MaxDuration,
			cfg.SmoothScroll.DurationPerPixel,
		)

		return nil
	}

	// A scroll arriving with the animation switched off must not be chased by
	// chunks scheduled before the reload.
	scrollAnim.stop()

	return scrollWheelNow(deltaX, deltaY, modifiers)
}

// scrollWheelNow injects the whole scroll in one go, which is what every
// caller got before smooth scroll existed and what a caller still gets with
// it switched off.
func scrollWheelNow(deltaX, deltaY int, modifiers action.Modifiers) error {
	hold, err := holdModifiers(modifiers)
	if err != nil {
		return err
	}

	defer hold.release()

	for _, event := range wheelEvents(deltaX, deltaY) {
		err := sendMouseInput(event.flags, event.data)
		if err != nil {
			return err
		}
	}

	return nil
}

// CurrentCursorPosition returns the current cursor location.
func CurrentCursorPosition() (image.Point, error) {
	return cursorPosition()
}
