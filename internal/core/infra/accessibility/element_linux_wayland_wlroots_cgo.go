//go:build linux && cgo

package accessibility

import (
	"image"
	"os"
	"sync"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

type wlrootsPointerState struct {
	mu                 sync.RWMutex
	mouseDown          bool
	mouseDownModifiers action.Modifiers
}

var globalWlrootsPointerState = &wlrootsPointerState{}

func wlrootsFocusedApplicationIdentity() (string, int) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", 0
	}

	// No standard Wayland protocol for querying focused app.
	// Fall through — the caller will try the XWayland fallback if DISPLAY is set.
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

func wlrootsLeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	err = linux.WaylandButtonEvent(point, linux.WlrBtnLeft, true)
	if err != nil {
		_ = wlrootsReleaseModifiers(modifiers)

		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = true
	globalWlrootsPointerState.mouseDownModifiers = modifiers
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsLeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	heldModifiers, hadMouseDown := wlrootsMouseDownModifiers()
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

	err := linux.WaylandButtonEvent(point, linux.WlrBtnLeft, false)
	if err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mouseDownModifiers = 0
	globalWlrootsPointerState.mu.Unlock()

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

func wlrootsLeftMouseUp() error {
	modifiers, hadMouseDown := wlrootsMouseDownModifiers()

	err := linux.WaylandButtonRelease(linux.WlrBtnLeft)
	if err != nil {
		return err
	}

	if hadMouseDown {
		_ = wlrootsReleaseModifiers(modifiers)
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mouseDownModifiers = 0
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsMouseDownModifiers() (action.Modifiers, bool) {
	globalWlrootsPointerState.mu.RLock()
	defer globalWlrootsPointerState.mu.RUnlock()

	return globalWlrootsPointerState.mouseDownModifiers, globalWlrootsPointerState.mouseDown
}

// wlrootsScrollScale mirrors the uinput scroll scaling constant so
// that both backends produce comparable scroll behavior from the same
// pixel-level delta values supplied by the scroll service
// (e.g. ScrollStep=50, ScrollStepHalf=500, ScrollStepFull=1000000).
const (
	wlrootsScrollScale     = 30
	wlrootsScrollMaxEvents = 50
	wlrootsScrollStep      = 30 // pixels per notch (matches uinput scrollScale)
)

func wlrootsScrollAtCursor(deltaX, deltaY int) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

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
