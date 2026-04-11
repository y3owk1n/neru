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

func wlrootsLeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	_ = modifiers // modifier injection not yet supported on Wayland

	original := wlrootsCurrentCursorPosition()
	if err := linux.WlrootsClick(point, linux.WlrBtnLeft); err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsRightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	_ = modifiers

	original := wlrootsCurrentCursorPosition()
	if err := linux.WlrootsClick(point, linux.WlrBtnRight); err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsMiddleClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	_ = modifiers

	original := wlrootsCurrentCursorPosition()
	if err := linux.WlrootsClick(point, linux.WlrBtnMiddle); err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WlrootsMoveCursorToPoint(original)
	}

	return nil
}

func wlrootsLeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	_ = modifiers

	if err := linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, true); err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = true
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsLeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	_ = modifiers

	if err := linux.WlrootsButtonEvent(point, linux.WlrBtnLeft, false); err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsLeftMouseUp() error {
	if err := linux.WlrootsButtonRelease(linux.WlrBtnLeft); err != nil {
		return err
	}

	globalWlrootsPointerState.mu.Lock()
	globalWlrootsPointerState.mouseDown = false
	globalWlrootsPointerState.mu.Unlock()

	return nil
}

func wlrootsScrollAtCursor(deltaX, deltaY int) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	// Vertical scroll (axis 0).
	if deltaY != 0 {
		if err := linux.WlrootsScroll(0, -deltaY); err != nil {
			return err
		}
	}

	// Horizontal scroll (axis 1).
	if deltaX != 0 {
		if err := linux.WlrootsScroll(1, deltaX); err != nil {
			return err
		}
	}

	return nil
}
