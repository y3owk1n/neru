//go:build linux && cgo

package linux

import (
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// globalWlrootsPointerState records the modifiers each held button was pressed
// with, so that the matching release can undo exactly those modifiers.
var globalWlrootsPointerState mousestate.Tracker

func wlrootsFocusedApplicationIdentity() (string, int) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", 0
	}

	// The wlr-foreign-toplevel-management protocol (wlroots + KWin/KDE) reports
	// the focused toplevel's app_id but not its PID, so the PID is 0. app_id is
	// the identifier used for per-app configuration lookups; when a caller needs
	// a real PID it falls back to the XWayland path if DISPLAY is set.
	appID, ok := linux.WaylandFocusedAppID()
	if ok && appID != "" {
		return appID, 0
	}

	// No focused app_id available (e.g. GNOME/Mutter has no such manager, or
	// nothing is focused yet). Fall through to the XWayland fallback.
	return "", 0
}

func wlrootsApplicationBundleIdentifier(pid int) string {
	_ = pid

	return ""
}

func wlrootsMoveMouseToPoint(point image.Point) error {
	return linux.WaylandMoveCursorToPoint(point)
}

func wlrootsCurrentCursorPosition() image.Point {
	pos, err := linux.WaylandCursorPosition()
	if err != nil {
		return image.Point{}
	}

	return pos
}

func wlrootsLeftClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnLeft)
}

func wlrootsRightClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnRight)
}

func wlrootsMiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnMiddle)
}

// wlrootsButton maps a domain mouse button to its wlroots button code.
func wlrootsButton(button action.MouseButton) int {
	switch button {
	case action.ButtonRight:
		return linux.WlrBtnRight
	case action.ButtonMiddle:
		return linux.WlrBtnMiddle
	case action.ButtonLeft:
		fallthrough
	default:
		return linux.WlrBtnLeft
	}
}

func wlrootsMouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	err = linux.WaylandButtonEvent(point, wlrootsButton(button), true)
	if err != nil {
		_ = wlrootsReleaseModifiers(modifiers)

		return err
	}

	globalWlrootsPointerState.SetDown(button, point, modifiers)

	return nil
}

func wlrootsMouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	heldModifiers, hadMouseDown := globalWlrootsPointerState.DownModifiers(button)
	if hadMouseDown {
		modifiers = heldModifiers
	} else {
		err := wlrootsPressModifiers(modifiers)
		if err != nil {
			return err
		}
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	err := linux.WaylandButtonEvent(point, wlrootsButton(button), false)
	if err != nil {
		return err
	}

	globalWlrootsPointerState.Clear(button)

	return nil
}

func wlrootsClickButtonAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
	button int,
) error {
	original := wlrootsCurrentCursorPosition()

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	err = linux.WaylandClick(point, button)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WaylandMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsPressModifiers(modifiers action.Modifiers) error {
	if modifiers.Has(action.ModShift) {
		err := linux.WaylandModifierEvent("shift", true)
		if err != nil {
			return err
		}
	}

	if modifiers.Has(action.ModCtrl) {
		err := linux.WaylandModifierEvent("ctrl", true)
		if err != nil {
			_ = linux.WaylandModifierEvent("shift", false)

			return err
		}
	}

	if modifiers.Has(action.ModAlt) {
		err := linux.WaylandModifierEvent("alt", true)
		if err != nil {
			_ = linux.WaylandModifierEvent("ctrl", false)
			_ = linux.WaylandModifierEvent("shift", false)

			return err
		}
	}

	if modifiers.Has(action.ModCmd) {
		err := linux.WaylandModifierEvent("cmd", true)
		if err != nil {
			_ = linux.WaylandModifierEvent("alt", false)
			_ = linux.WaylandModifierEvent("ctrl", false)
			_ = linux.WaylandModifierEvent("shift", false)

			return err
		}
	}

	return nil
}

func wlrootsReleaseModifiers(modifiers action.Modifiers) error {
	var firstErr error

	release := func(modifier string) {
		err := linux.WaylandModifierEvent(modifier, false)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if modifiers.Has(action.ModCmd) {
		release("cmd")
	}

	if modifiers.Has(action.ModAlt) {
		release("alt")
	}

	if modifiers.Has(action.ModCtrl) {
		release("ctrl")
	}

	if modifiers.Has(action.ModShift) {
		release("shift")
	}

	return firstErr
}

func wlrootsMouseUp(button action.MouseButton) error {
	modifiers, hadMouseDown := globalWlrootsPointerState.DownModifiers(button)

	err := linux.WaylandButtonRelease(wlrootsButton(button))
	if err != nil {
		return err
	}

	if hadMouseDown {
		_ = wlrootsReleaseModifiers(modifiers)
	}

	globalWlrootsPointerState.Clear(button)

	return nil
}

// wlrootsScrollScale mirrors the uinput scroll scaling constant so
// that both backends produce comparable scroll behavior from the same
// pixel-level delta values supplied by the scroll service
// (e.g. ScrollStep=50, ScrollStepHalf=500, ScrollStepFull=1000000).
const (
	wlrootsScrollScale     = scrollPixelsPerNotch
	wlrootsScrollMaxEvents = 50
	wlrootsScrollStep      = scrollPixelsPerNotch // pixels per notch
)

// wlrootsScrollAtCursor emits the scroll on the wlroots virtual pointer, with
// modifiers held on the virtual keyboard (libei on KDE) for its duration.
//
// Both halves go out through the same seat, which is the whole reason a
// modified scroll is routed here rather than through the faster uinput batch.
// The release only lets go of what this call pressed, so a modifier the user is
// physically holding survives it.
func wlrootsScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	if deltaY != 0 {
		err := wlrootsScrollAxis(0, deltaY)
		if err != nil {
			return err
		}
	}

	if deltaX != 0 {
		err := wlrootsScrollAxis(1, deltaX)
		if err != nil {
			return err
		}
	}

	return nil
}

// waylandScrollSession injects an animated scroll on Wayland as continuous axis
// events, holding any modifiers for the length of the animation.
//
// It uses the continuous axis rather than the discrete one the unanimated path
// sends, and that choice is the whole reason smooth scroll works here: an axis
// event with no discrete step count reaches the focused client as the fraction
// it carries, while a discrete one declares a wheel notch that the compositor
// may hold back until whole notches accumulate. Both wlroots (through
// zwlr_virtual_pointer) and KWin (through libei's pixel-precise scroll delta)
// pass the fraction through; linux.WaylandScrollContinuous picks between them.
type waylandScrollSession struct {
	modifiers action.Modifiers
}

// waylandScrollBackendAvailable answers whether an animated scroll could inject
// here, without touching the compositor to find out.
func waylandScrollBackendAvailable() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	return nil
}

func newWaylandScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	err := waylandScrollBackendAvailable()
	if err != nil {
		return nil, err
	}

	if modifiers != 0 {
		pressErr := wlrootsPressModifiers(modifiers)
		if pressErr != nil {
			return nil, pressErr
		}
	}

	return &waylandScrollSession{modifiers: modifiers}, nil
}

// granularity is zero: a Wayland axis value is a distance, not a step count, so
// there is no unit to round to.
func (s *waylandScrollSession) granularity() float64 { return 0 }

func (s *waylandScrollSession) inject(deltaX, deltaY float64) error {
	if deltaY != 0 {
		// Wayland axis convention: positive = scroll down. Application
		// convention: positive delta = scroll up. Same negation the discrete
		// path applies in wlrootsScrollAxis.
		err := linux.WaylandScrollContinuous(0, -deltaY)
		if err != nil {
			return err
		}
	}

	if deltaX != 0 {
		return linux.WaylandScrollContinuous(1, deltaX)
	}

	return nil
}

// close releases only what this session pressed, so a modifier the user is
// physically holding survives the animation.
func (s *waylandScrollSession) close() {
	if s.modifiers != 0 {
		_ = wlrootsReleaseModifiers(s.modifiers)
	}
}

// wlrootsScrollAxis sends Wayland axis events for one axis.
// Each event carries 1 notch (axis_discrete=±1, axis_value120=±120) to
// match what a physical mouse wheel produces — no toolkit clipping.
// Events are sent in batches of wlrootsScrollMaxEvents to avoid flooding
// the compositor socket.
//
// Wayland axis convention: positive = scroll down (axis 0) / right (axis 1).
// Application  convention: positive delta = scroll up (axis 0) / right (axis 1).
// Vertical axis sign is negated to convert between the two.
func wlrootsScrollAxis(axis int, delta int) error {
	totalNotches := abs(delta) / wlrootsScrollScale
	if totalNotches == 0 {
		totalNotches = 1
	}

	negate := axis == 0

	step := wlrootsScrollStep
	if negate {
		step = -step
	}

	disc := 1
	if delta < 0 {
		step = -step
		disc = -disc
	}

	deltas := make([]int, 0, wlrootsScrollMaxEvents)
	discretes := make([]int, 0, wlrootsScrollMaxEvents)
	remaining := totalNotches

	for remaining > 0 {
		deltas = append(deltas, step)
		discretes = append(discretes, disc)
		remaining--

		if len(deltas) >= wlrootsScrollMaxEvents || remaining == 0 {
			err := linux.WlrootsScrollBatch(axis, deltas, discretes)
			if err != nil {
				return err
			}

			deltas = deltas[:0]
			discretes = discretes[:0]
		}
	}

	return nil
}
