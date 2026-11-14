//go:build linux

package inject

import (
	"image"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	linuxplatform "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// linuxHeldButtons records which mouse buttons Neru is currently holding down.
var linuxHeldButtons mousestate.Tracker

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// IsMouseButtonDown returns whether the given mouse button is held down.
func IsMouseButtonDown(button action.MouseButton) bool {
	return linuxHeldButtons.IsDown(button)
}

// EnsureMouseUp releases every mouse button Neru is currently holding down.
func EnsureMouseUp() {
	for _, button := range linuxHeldButtons.HeldButtons() {
		_ = MouseUp(button)
	}
}

// MoveMouseToPoint moves the cursor to point.
func MoveMouseToPoint(point image.Point) error {
	switch currentLinuxBackend() {
	case linuxBackendX11:
		return x11MoveMouseToPoint(point)
	case linuxBackendWayland:
		return wlrootsMoveMouseToPoint(point)
	case linuxBackendUnknown:
	}

	return noMouseBackend("move")
}

// noMouseBackend is what a mouse action answers with neither X11 nor Wayland
// detected. nil would report a click or drag that never happened.
func noMouseBackend(what string) error {
	return derrors.Newf(derrors.CodeNotSupported, "%s requires an X11 or Wayland backend", what)
}

// LeftClickAtPoint performs a left click.
func LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11LeftClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsLeftClickAtPoint(point, restoreCursor, modifiers)
	}

	return noMouseBackend("left click")
}

// RightClickAtPoint performs a right click.
func RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11RightClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsRightClickAtPoint(point, restoreCursor, modifiers)
	}

	return noMouseBackend("right click")
}

// MiddleClickAtPoint performs a middle click.
func MiddleClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11MiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsMiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	return noMouseBackend("middle click")
}

// MouseDownAtPoint presses and holds the given mouse button at the point.
func MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	if currentLinuxBackend() == linuxBackendX11 {
		err := x11MouseDownAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.SetDown(button, point, modifiers)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseDownAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.SetDown(button, point, modifiers)
		}

		return err
	}

	return noMouseBackend("mouse down")
}

// MouseUpAtPoint releases the given mouse button at the point.
//
// A release undoes the modifiers recorded at press time, not the ones this
// call was given, or a bare "up" after a modified press leaves the modifier
// held. wlroots applies the same rule internally. X11 releases the keys its
// own press touched and falls back on this record for a release with no press
// behind it.
func MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	if currentLinuxBackend() == linuxBackendX11 {
		if heldModifiers, wasHeld := linuxHeldButtons.DownModifiers(button); wasHeld {
			modifiers = heldModifiers
		}

		err := x11MouseUpAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseUpAtPoint(point, button, modifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	return noMouseBackend("mouse up")
}

// MouseUp releases the given mouse button at the cursor, together with the
// modifiers its press is holding. This is the idle-cleanup path, so a modified
// press that is never explicitly released still leaves nothing held.
func MouseUp(button action.MouseButton) error {
	if currentLinuxBackend() == linuxBackendX11 {
		heldModifiers, _ := linuxHeldButtons.DownModifiers(button)

		err := x11MouseUp(button, heldModifiers)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsMouseUp(button)
		if err == nil {
			linuxHeldButtons.Clear(button)
		}

		return err
	}

	return noMouseBackend("mouse up")
}

// scrollPixelsPerNotch is how many pixels of a caller's delta make one wheel
// notch. Every Linux injection path shares it, so a binding travels the same
// distance whichever backend answers.
const scrollPixelsPerNotch = 30

// scrollNotches is how many wheel notches a scroll of delta pixels is, rounded
// to the nearest and never fewer than one. Truncating left a default 50 px
// step one notch short. Every unanimated path and the animated total share
// it, so both travel the same distance.
func scrollNotches(delta int) int {
	return max((abs(delta)+scrollPixelsPerNotch/2)/scrollPixelsPerNotch, 1)
}

// ScrollAtCursor scrolls at the cursor, presenting modifiers as held.
//
// The caller's deltaX is positive-left, Neru's shared convention. Every Linux
// layer below reads positive as right (X11 button 7, REL_HWHEEL, the Wayland
// horizontal axis), so this negates deltaX once.
//
// On Wayland a scroll goes out on the wlroots virtual pointer, or libei on KDE
// and GNOME. The uinput wheel is a real device to the compositor, which applies
// its mouse settings to it, so with natural scrolling on a scroll down would go
// up. A modifier is a real key press on the virtual keyboard or libei, on the
// same seat as the scroll.
//
// Hyprland is the exception. Its virtual-pointer scroll goes missing (#1474),
// so there the uinput wheel carries the scroll and the modifier is pressed
// beside it.
//
// With smooth_scroll.enabled the animator sends eased chunks, as on macOS,
// modified scrolls and Hyprland included. The backend is checked before the
// handoff, because a refusal on the animation's worker goroutine has nobody to
// report to and would be a silent no-op.
func ScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	deltaX = -deltaX

	anim, ok := linuxplatform.CurrentAnimation()
	if ok && anim.Scroll.Enabled {
		err := scrollBackendAvailable()
		if err != nil {
			return err
		}

		if deltaX == 0 && deltaY == 0 {
			return nil
		}

		scrollAnim.animate(
			deltaX,
			deltaY,
			modifiers,
			anim.Scroll.Steps,
			anim.Scroll.MaxDuration,
			anim.Scroll.DurationPerPixel,
		)

		return nil
	}

	// A scroll arriving with the animation switched off must not be chased by
	// chunks scheduled before the reload.
	scrollAnim.stop()

	return scrollAtCursorNow(deltaX, deltaY, modifiers)
}

// scrollAtCursorNow injects the whole scroll in one go, as with smooth scroll
// off.
func scrollAtCursorNow(deltaX, deltaY int, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11ScrollAtCursor(deltaX, deltaY, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		// Nothing to travel. The guard sits here, not at the top, because the
		// no-backend arm below still owes a modified scroll a refusal.
		if deltaX == 0 && deltaY == 0 {
			return nil
		}

		// Release the held chord first, or a ctrl+j scroll goes out as ctrl+scroll.
		restore := liftPhysicalModifiers()
		defer restore()

		if !hyprlandKeepsUinputScroll() {
			// Uncapped, unlike X11's 50-click ceiling, so a go_bottom travels as far
			// as it does on Hyprland's uinput wheel.
			return wlrootsScrollAtCursor(deltaX, deltaY, modifiers)
		}

		if modifiers != 0 {
			pressed, err := pressWaylandModifiers(modifiers)
			if err != nil {
				return err
			}

			// Both sides wait. ScrollDeviceScrollBatch returns once the kernel takes
			// the write, not once the compositor reads it, so an immediate release
			// could lift the modifier from under notches still in flight.
			defer func() {
				waitForScrollDelivery()

				_ = releaseWaylandModifiers(pressed)
			}()

			waitForWaylandModifierPress()
		}

		// maxBatchEvents caps uinput events per write so neither the kernel evdev
		// buffer (~8192 bytes) nor the Wayland socket overflows.
		const maxBatchEvents = 50

		// fallbackCause is the uinput error that sends the rest of the scroll
		// to the virtual pointer, for the one-time warning below.
		var fallbackCause error

		sendScaledScroll := func(axis int, delta int) int {
			if delta == 0 {
				return 0
			}

			totalNotches := scrollNotches(delta)

			remainingNotches := totalNotches
			batch := make([]int, 0, maxBatchEvents)

			value := 1
			if delta < 0 {
				value = -1
			}

			for remainingNotches > 0 {
				batch = append(batch, value)
				remainingNotches--

				if len(batch) >= maxBatchEvents || remainingNotches == 0 {
					err := uinputScrollBatch(axis, batch)
					if err != nil {
						// uinput failed. Add back the unsent notches so the virtual
						// pointer retries only what was not sent.
						remainingNotches += len(batch)
						fallbackCause = err

						break
					}

					batch = batch[:0]
				}
			}

			return uinputScrollRemainder(
				delta, totalNotches, remainingNotches, scrollPixelsPerNotch,
			)
		}

		remainY := sendScaledScroll(uinputScrollAxisVertical, deltaY)
		remainX := sendScaledScroll(uinputScrollAxisHorizontal, deltaX)

		if remainY == 0 && remainX == 0 {
			return nil
		}

		warnUinputScrollFallback(fallbackCause)

		return wlrootsScrollAtCursor(remainX, remainY, 0)
	}

	// No backend. An unmodified scroll is a silent no-op here, but a modified one
	// is refused so a missing zoom is not read as a broken binding.
	if modifiers != 0 {
		return derrors.Newf(
			derrors.CodeNotSupported,
			"modified scroll (%s) is not supported without an X11 or Wayland backend",
			modifiers,
		)
	}

	return nil
}

// uinputScrollRemainder is the part of a scroll the uinput wheel did not
// send, in the caller's pixels, for the virtual pointer to finish.
//
// It is zero whenever every notch went out. The 10 px a 50 px step leaves over
// two 30 px notches is rounding. Handing it on added an opposite-signed notch
// per press on Hyprland, which Chromium and Electron summed to nothing. When
// notches remain, the pixels round to exactly the unsent count.
func uinputScrollRemainder(delta, totalNotches, remainingNotches, scale int) int {
	if remainingNotches == 0 {
		return 0
	}

	pixelsSent := (totalNotches - remainingNotches) * scale
	if delta > 0 {
		return max(delta-pixelsSent, 0)
	}

	return min(delta+pixelsSent, 0)
}

// wlrootsScrollNotch is one virtual-pointer wheel notch, the pixel step and
// discrete count in Wayland's convention (positive is down and right). The
// caller's vertical delta is positive-up, so that axis flips both together.
// A count disagreeing with the step splits axis and axis_value120 readers.
func wlrootsScrollNotch(axis, delta, step int) (int, int) {
	if delta < 0 {
		step = -step
	}

	if axis == uinputScrollAxisVertical {
		step = -step
	}

	disc := 1
	if step < 0 {
		disc = -1
	}

	return step, disc
}

// uinputScrollFallbackOnce keeps the fallback warning to one line per
// process. The condition is a session fact, and every scroll would repeat it.
var uinputScrollFallbackOnce sync.Once

// warnUinputScrollFallback says, once, that scrolling left the uinput wheel
// for the virtual pointer and why. Chromium and Electron on Hyprland ignore
// the virtual pointer, so a root-only /dev/uinput reads as "scroll works in
// Firefox but not Discord". cause is the batch error, a device creation
// failure or a later write failure.
func warnUinputScrollFallback(cause error) {
	uinputScrollFallbackOnce.Do(func() {
		currentLogger().Warn(
			"Scrolling through the wlroots virtual pointer instead of uinput; "+
				"some clients ignore that path (grant write access to /dev/uinput, "+
				"see docs/guide/linux.md)",
			zap.Error(cause),
		)
	})
}

// hyprlandKeepsUinputScroll reports whether a scroll on this session goes
// through the uinput wheel, with any modifier pressed beside it, rather than
// the wlroots virtual pointer.
func hyprlandKeepsUinputScroll() bool {
	return currentLinuxBackend() == linuxBackendWayland && platform.IsHyprlandSession()
}

// uinput scroll axes, in the vocabulary ScrollDeviceScrollBatch takes.
const (
	uinputScrollAxisVertical   = 0
	uinputScrollAxisHorizontal = 1
)

// The virtual keyboard and the uinput scroll device are separate input sources
// with no ordering between them. A notch that overtakes the modifier arrives
// unmodified, and one that outlives its release arrives modified. So both
// sides of a hold wait.
const (
	// modifierSyncTimeout bounds the press barrier. It is not a delay, since a
	// responsive compositor costs a millisecond. It caps how long a stalled one
	// can block the dispatch goroutine.
	modifierSyncTimeout = 100 * time.Millisecond
	// modifierSettleDelay is the fixed wait used where no barrier exists.
	modifierSettleDelay = 5 * time.Millisecond
)

// waitForWaylandModifierPress blocks until the compositor has applied the
// just-pressed modifier. wl_display.sync answers once every earlier request
// is processed, a real barrier the uinput notches cannot overtake. The fixed
// delay covers the cases with nothing to ask, such as no cgo, a stalled
// compositor, or no virtual keyboard.
func waitForWaylandModifierPress() {
	if linuxplatform.WaylandSyncModifiers(modifierSyncTimeout) {
		return
	}

	time.Sleep(modifierSettleDelay)
}

// waitForScrollDelivery gives uinput notches already written time to be read
// before their modifier is released. There is no barrier here. Syncing the
// Wayland connection says nothing about how far the compositor has read a
// kernel evdev device, so this is a fixed, heuristic wait.
func waitForScrollDelivery() {
	time.Sleep(modifierSettleDelay)
}

// Seams over the virtual-keyboard event and the uinput scroll write, so a test
// can read back what was emitted and fail either to reach the unwind and
// fallback paths without a compositor.
var (
	waylandModifierEvent = linuxplatform.WaylandModifierEvent
	uinputScrollBatch    = eventtaplinux.ScrollDeviceScrollBatch
)

// waylandModifierKeys is the modifier vocabulary the wlroots virtual keyboard
// speaks, in press order. A release walks it backwards, so a set going down
// shift-first comes back up shift-last.
var waylandModifierKeys = []struct {
	modifier action.Modifiers
	name     string
}{
	{action.ModShift, "shift"},
	{action.ModCtrl, "ctrl"},
	{action.ModAlt, "alt"},
	{action.ModCmd, "cmd"},
}

// pressWaylandModifiers presses each requested modifier on the virtual keyboard
// and reports the set that actually went down, which the caller has to release.
//
// A failure unwinds that set and nothing else. WaylandModifierEvent refcounts
// holders and emits a real key-up at zero, so releasing a modifier this call
// never pressed would let go of one the user is physically holding.
func pressWaylandModifiers(modifiers action.Modifiers) (action.Modifiers, error) {
	var pressed action.Modifiers

	for _, key := range waylandModifierKeys {
		if !modifiers.Has(key.modifier) {
			continue
		}

		err := waylandModifierEvent(key.name, true)
		if err != nil {
			_ = releaseWaylandModifiers(pressed)

			return 0, err
		}

		pressed |= key.modifier
	}

	return pressed, nil
}

// releaseWaylandModifiers releases exactly the set pressWaylandModifiers
// reported, reporting the first failure and letting go of the rest regardless.
// Stopping at the first error would leave the modifiers after it held.
func releaseWaylandModifiers(pressed action.Modifiers) error {
	_, err := releaseWaylandModifiersRemaining(pressed)

	return err
}

// releaseWaylandModifiersRemaining is releaseWaylandModifiers reporting the
// modifiers still held because their release failed, for a caller that keeps
// a record to retry from.
func releaseWaylandModifiersRemaining(pressed action.Modifiers) (action.Modifiers, error) {
	var (
		firstErr  error
		remaining action.Modifiers
	)

	for _, key := range slices.Backward(waylandModifierKeys) {
		if !pressed.Has(key.modifier) {
			continue
		}

		err := waylandModifierEvent(key.name, false)
		if err != nil {
			remaining |= key.modifier

			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return remaining, firstErr
}

type linuxBackend string

const (
	linuxBackendUnknown linuxBackend = "unknown"
	linuxBackendX11     linuxBackend = "x11"
	linuxBackendWayland linuxBackend = "wayland"
)

// currentLinuxBackend maps platform.DetectLinuxBackend so compositor-family
// detection is consistent across layers.
func currentLinuxBackend() linuxBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxBackendX11
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE, platform.BackendWaylandCOSMIC,
		platform.BackendWaylandGNOME:
		return linuxBackendWayland
	case platform.BackendUnknown, platform.BackendWaylandOther:
		return linuxBackendUnknown
	}

	return linuxBackendUnknown
}
