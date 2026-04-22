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
	return linux.WlrootsMoveCursorToPoint(point)
}

func wlrootsCurrentCursorPosition() image.Point {
	pos, err := linux.WlrootsCursorPosition()
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

	err = linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, true)
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

	err := linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, false)
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

	err = linux.WlrootsClick(point, button)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsPressModifiers(modifiers action.Modifiers) error {
	if modifiers.Has(action.ModShift) {
		if err := linux.WlrootsModifierEvent("shift", true); err != nil {
			return err
		}
	}
	if modifiers.Has(action.ModCtrl) {
		if err := linux.WlrootsModifierEvent("ctrl", true); err != nil {
			_ = linux.WlrootsModifierEvent("shift", false)

			return err
		}
	}
	if modifiers.Has(action.ModAlt) {
		if err := linux.WlrootsModifierEvent("alt", true); err != nil {
			_ = linux.WlrootsModifierEvent("ctrl", false)
			_ = linux.WlrootsModifierEvent("shift", false)

			return err
		}
	}
	if modifiers.Has(action.ModCmd) {
		if err := linux.WlrootsModifierEvent("cmd", true); err != nil {
			_ = linux.WlrootsModifierEvent("alt", false)
			_ = linux.WlrootsModifierEvent("ctrl", false)
			_ = linux.WlrootsModifierEvent("shift", false)

			return err
		}
	}

	return nil
}

func wlrootsReleaseModifiers(modifiers action.Modifiers) error {
	var firstErr error
	release := func(modifier string) {
		if err := linux.WlrootsModifierEvent(modifier, false); err != nil && firstErr == nil {
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

	err := linux.WlrootsButtonRelease(linux.WlrBtnLeft)
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

// wlrootsScrollScale and wlrootsScrollMaxEvents mirror the X11 scroll
// scaling constants so that both backends produce comparable scroll
// behavior from the same pixel-level delta values supplied by the
// scroll service (e.g. ScrollStep=50, ScrollStepHalf=500,
// ScrollStepFull=1000000).
const (
	wlrootsScrollScale     = 30
	wlrootsScrollMaxEvents = 50
	wlrootsScrollStep      = 15 // pixels per Wayland axis event
)

func wlrootsScrollAtCursor(deltaX, deltaY int) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	// Vertical scroll (axis 0).
	// Incoming deltas are pixel-level values from the scroll service config.
	// We scale them to a bounded number of small Wayland axis events, just
	// like the X11 backend converts them to discrete button clicks, to
	// avoid sending a single enormous axis event for ScrollStepFull.
	if deltaY != 0 {
		events := wlrootsScrollEvents(deltaY)

		// Wayland axis convention: positive = scroll down.
		// Application convention: positive deltaY = scroll up.
		// Negate to convert between the two conventions.
		step := -wlrootsScrollStep
		if deltaY < 0 {
			step = wlrootsScrollStep
		}

		for range events {
			err := linux.WlrootsScroll(0, step)
			if err != nil {
				return err
			}
		}
	}

	// Horizontal scroll (axis 1).
	// Wayland axis convention: positive = scroll right.
	// Application convention: positive deltaX = scroll right.
	// No negation needed.
	if deltaX != 0 {
		events := wlrootsScrollEvents(deltaX)

		step := wlrootsScrollStep
		if deltaX < 0 {
			step = -wlrootsScrollStep
		}

		for range events {
			err := linux.WlrootsScroll(1, step)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// wlrootsScrollEvents converts a raw pixel delta to a bounded number of
// scroll events, matching the X11 backend's scaling logic.
func wlrootsScrollEvents(delta int) int {
	if delta < 0 {
		delta = -delta
	}

	events := delta / wlrootsScrollScale
	if events == 0 {
		events = 1
	}

	if events > wlrootsScrollMaxEvents {
		events = wlrootsScrollMaxEvents
	}

	return events
}
