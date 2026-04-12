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
	mu        sync.RWMutex
	mouseDown bool
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
	_ = modifiers // modifier injection not yet supported on Wayland

	original := wlrootsCurrentCursorPosition()

	err := linux.WlrootsClick(point, linux.WlrBtnLeft)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsRightClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_ = modifiers

	original := wlrootsCurrentCursorPosition()

	err := linux.WlrootsClick(point, linux.WlrBtnRight)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsMiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_ = modifiers

	original := wlrootsCurrentCursorPosition()

	err := linux.WlrootsClick(point, linux.WlrBtnMiddle)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsLeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	_ = modifiers

	err := linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, true)
	if err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = true
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsLeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	_ = modifiers

	err := linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, false)
	if err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsLeftMouseUp() error {
	err := linux.WlrootsButtonRelease(linux.WlrBtnLeft)
	if err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

// wlrootsScrollScale and wlrootsScrollMaxEvents mirror the X11 scroll
// scaling constants so that both backends produce comparable scroll
// behaviour from the same pixel-level delta values supplied by the
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
