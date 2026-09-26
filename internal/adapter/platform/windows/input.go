//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"math"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// heldButtons records which mouse buttons Neru is currently holding down. A
// cursor move while any is down has to reach the input pipeline as a drag
// motion (see moveCursorTo), so the press and release keep it current.
var heldButtons mousestate.Tracker

// Mouse and keyboard synthesis via SendInput.
// Does not implement accessibility element actions.
const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseeventfMove        = 0x0001
	mouseeventfLeftDown    = 0x0002
	mouseeventfLeftUp      = 0x0004
	mouseeventfRightDown   = 0x0008
	mouseeventfRightUp     = 0x0010
	mouseeventfMiddleDown  = 0x0020
	mouseeventfMiddleUp    = 0x0040
	mouseeventfWheel       = 0x0800
	mouseeventfHWheel      = 0x1000
	mouseeventfAbsolute    = 0x8000
	mouseeventfVirtualDesk = 0x4000

	// absoluteCoordinateRange is the span an absolute mouse event's dx and dy
	// address across the virtual desktop. Windows maps a value back to a
	// pixel as value * width / 65536, floored.
	absoluteCoordinateRange = 65536

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79

	// dragGlideSteps and dragGlideInterval shape the motion a warp becomes
	// while a button is held. Applications do not turn a single move into a
	// drag: a press, one jump, and a release select nothing in Notepad and
	// only sometimes in Edge, whether the jump is SetCursorPos or an
	// injected absolute move. Interpolated motion between the two points
	// selects the whole range every time in both, so a held warp is spread
	// over this many steps (dragGlidePath).
	dragGlideSteps    = 20
	dragGlideInterval = 6 * time.Millisecond

	// dragLandingApproach is how much of a drag step is left to the short
	// relative move that ends it (dragApproachPoint). The pointer speed and
	// threshold settings scale relative motion, and Windows doubles a move
	// whose distance along either axis is greater than the first threshold, 6
	// pixels by default. The warp that follows each move corrects its drift,
	// but the last move of a drag is the one the application reads as the end
	// of it, and no warp can take that back, so every step ends with a move
	// both axes of which stay under those thresholds. Rounding the approach
	// point to a pixel can stretch the move past this figure diagonally, never
	// past it on an axis.
	dragLandingApproach = 4

	// dragReleaseSettle is how long a release waits after its last motion
	// so the application processes the move at the release point before
	// the button-up. Only a drag this process holds pays it, on a mode-exit
	// path, so it never sits on a keystroke.
	dragReleaseSettle    = 20 * time.Millisecond
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
	heldButtons.SetDown(button, point, modifiers)

	return nil
}

// IsMouseButtonDown returns whether Neru is holding the given button down.
func IsMouseButtonDown(button action.MouseButton) bool {
	return heldButtons.IsDown(button)
}

// HeldMouseButtons returns every button Neru is holding down.
func HeldMouseButtons() []action.MouseButton {
	return heldButtons.HeldButtons()
}

// MouseUp releases the given button at the given point, undoing the hold its
// press kept, or presenting modifiers for the length of the release event
// when no press of this process is behind it. Releasing a drag does not depend
// on the move to the release point succeeding (dragReleaseAt).
func MouseUp(point image.Point, button action.MouseButton, modifiers action.Modifiers) error {
	hold, err := resumeModifierHold(button, modifiers)
	if err != nil {
		return err
	}

	defer hold.release()

	var released bool

	flags := flagsForButton(button).up
	if heldButtons.IsDown(button) {
		released, err = dragReleaseAt(point, flags)
	} else {
		err = buttonEventAt(point, flags)
		released = err == nil
	}

	// The tracker follows the button-up, not the error: a release that went out
	// after a failed move still left the button up, and leaving it recorded as
	// held would make every later click a drag.
	if released {
		heldButtons.Clear(button)
	}

	return err
}

// dragGlideTo moves the pointer from where it is to target along the glide
// path, one step every dragGlideInterval, ending on target exactly.
func dragGlideTo(target image.Point) error {
	from, err := cursorPosition()
	if err != nil {
		return warpCursor(target)
	}

	path := dragGlidePath(from, target, dragGlideSteps)
	for index, step := range path {
		err = dragStepTo(step)
		if err != nil {
			return err
		}

		if index < len(path)-1 {
			time.Sleep(dragGlideInterval)
		}
	}

	return nil
}

// dragStepTo moves the pointer one drag step onto point. With nothing held
// there is no drag for an application to read, so it warps instead.
//
// A WinUI text control, and Windows Terminal's is one, extends a selection
// only on relative pointer motion. It reads nothing out of SetCursorPos or an
// absolute injected move, however finely Neru interpolates either one, though
// it does register the press, so a drag made of absolute motion selects the
// single character under it. A Win32 control takes the selection from the two
// endpoints instead, so absolute motion is enough for it.
//
// The step goes out as two relative moves, the second no longer than
// dragLandingApproach, because the move that ends a step may also be the move
// that ends the whole drag. Every caller lands on a pixel through here, the
// smooth-cursor animator included, and an animator's own step is as long as
// its interpolation makes it.
func dragStepTo(point image.Point) error {
	if !heldButtons.AnyDown() {
		return warpCursor(point)
	}

	from, err := cursorPosition()
	if err != nil {
		return warpCursor(point)
	}

	approach, split := dragApproachPoint(from, point)
	if split {
		err = dragMotionOnto(from, approach)
		if err != nil {
			return err
		}

		from = approach
	}

	return dragMotionOnto(from, point)
}

// dragMotionOnto posts the relative motion from one pixel to another, then
// warps to correct whatever the pointer thresholds made of the delta, so the
// error cannot accumulate over the moves that follow.
//
// The relative move's failure reaches the caller, unlike the redraw warpCursor
// posts. The redraw is cosmetic; this is the motion the drag is made of, and a
// caller that releases the button believing the drag happened would select the
// single character under the press instead.
func dragMotionOnto(from, onto image.Point) error {
	if from != onto {
		err := dragMotionBy(onto.Sub(from))
		if err != nil {
			return err
		}
	}

	return warpCursor(onto)
}

// dragApproachPoint returns the pixel a drag step passes through before landing
// on target, leaving dragLandingApproach of travel for the move that lands. A
// step that short already, or one whose approach rounds onto an endpoint, lands
// in a single move and reports false.
func dragApproachPoint(from, target image.Point) (image.Point, bool) {
	total := math.Hypot(float64(target.X-from.X), float64(target.Y-from.Y))
	if total <= dragLandingApproach {
		return image.Point{}, false
	}

	approach := pointAlong(from, target, (total-dragLandingApproach)/total)
	if approach == from || approach == target {
		return image.Point{}, false
	}

	return approach, true
}

// dragGlidePath returns the points a glide from one point to another passes
// through, target last, in the given number of even steps. A glide of no
// distance has no path. Each point is landed on by dragStepTo, which is what
// keeps the motion onto it short.
func dragGlidePath(from, target image.Point, steps int) []image.Point {
	if from == target || steps < 1 {
		return nil
	}

	points := make([]image.Point, 0, steps)

	for i := 1; i < steps; i++ {
		points = appendGlideStep(
			points,
			from,
			pointAlong(from, target, float64(i)/float64(steps)),
		)
	}

	return appendGlideStep(points, from, target)
}

// appendGlideStep adds one glide step to the path, unless it is the pixel the
// path is already on. A glide shorter than its step count rounds several steps
// onto the same pixel, and posting them would sleep dragGlideInterval for each
// one while no application sees any motion.
func appendGlideStep(points []image.Point, from, step image.Point) []image.Point {
	current := from
	if len(points) > 0 {
		current = points[len(points)-1]
	}

	if step == current {
		return points
	}

	return append(points, step)
}

// pointAlong returns the point the given fraction of the way from one point to
// another, rounded to the nearest pixel.
func pointAlong(from, target image.Point, fraction float64) image.Point {
	return image.Point{
		X: from.X + int(math.Round(float64(target.X-from.X)*fraction)),
		Y: from.Y + int(math.Round(float64(target.Y-from.Y)*fraction)),
	}
}

// dragMotionBy posts one relative MOUSEEVENTF_MOVE of delta through SendInput.
// This is the motion an application reads a drag out of. dragStepTo says why a
// drag needs it, and why Windows does not deliver the delta verbatim.
func dragMotionBy(delta image.Point) error {
	return sendMouseMove(mouseeventfMove, int32(delta.X), int32(delta.Y))
}

// dragMotionTo posts one absolute MOUSEEVENTF_MOVE at point through SendInput.
//
// While a button this process holds is down, Windows updates the pointer's
// position for SetCursorPos but does not redraw the pointer image: the drag
// lands and GetCursorPos reports the target, and the arrow on screen stays
// where the press was. An injected move at the same pixel is what redraws
// it. The absolute coordinate is chosen so Windows floors it back to the
// exact pixel, and it follows the warp so the pointer is drawn where it is.
func dragMotionTo(point image.Point) error {
	desktop := virtualScreenMetrics()

	return sendMouseMove(
		mouseeventfMove|mouseeventfAbsolute|mouseeventfVirtualDesk,
		absoluteCoordinate(point.X, desktop.Min.X, desktop.Dx()),
		absoluteCoordinate(point.Y, desktop.Min.Y, desktop.Dy()),
	)
}

// sendMouseMove posts one MOUSEEVENTF_MOVE with the given deltas. The flags
// decide what they mean: absolute virtual-desktop coordinates, or a relative
// delta.
func sendMouseMove(flags uint32, deltaX, deltaY int32) error {
	var event input

	event.inputType = inputMouse
	event.mi.dwFlags = flags
	event.mi.dx = deltaX
	event.mi.dy = deltaY

	return sendOneInput(unsafe.Pointer(&event), unsafe.Sizeof(event))
}

// virtualScreenMetrics reads the virtual desktop rectangle from the same
// system metrics Windows maps absolute mouse coordinates against.
func virtualScreenMetrics() image.Rectangle {
	left, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	top, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	width, _, _ := procGetSystemMetrics.Call(smCxVirtualScreen)
	height, _, _ := procGetSystemMetrics.Call(smCyVirtualScreen)

	origin := image.Point{X: int(int32(left)), Y: int(int32(top))}

	return image.Rectangle{
		Min: origin,
		Max: origin.Add(image.Point{X: int(int32(width)), Y: int(int32(height))}),
	}
}

// absoluteCoordinate maps a pixel on one axis of the virtual desktop onto the
// absolute range so that Windows' floor(value * size / 65536) lands on that
// pixel again: the smallest value whose product reaches the pixel.
func absoluteCoordinate(pixel, origin, size int) int32 {
	if size <= 0 {
		return 0
	}

	offset := min(max(pixel-origin, 0), size-1)
	value := (offset*absoluteCoordinateRange + size - 1) / size

	return int32(min(value, absoluteCoordinateRange-1))
}

// dragReleaseAt posts the release of a drag this process holds. It brings the
// pointer to point, gives the application dragReleaseSettle to process the
// motion, and only then releases. It reports whether the button-up went out,
// and the first error behind it.
//
// A failed move does not hold the release back. Losing the drag's last stretch
// selects the wrong range, which is recoverable; skipping the button-up leaves
// a button held that every later click inherits, and EnsureMouseUp cannot get
// it back, because its own MouseUp comes through here and fails the same way.
// The release's own error outranks the move's.
func dragReleaseAt(point image.Point, flags uint32) (bool, error) {
	moveErr := moveCursorTo(point)

	time.Sleep(dragReleaseSettle)

	err := sendMouseInput(flags, 0)
	if err != nil {
		return false, err
	}

	return true, moveErr
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
