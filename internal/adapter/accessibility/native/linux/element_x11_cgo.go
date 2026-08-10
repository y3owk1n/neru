//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <stdlib.h>
#include "../../../platform/linux/x11_accessibility.h"
*/
import "C"

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	mouseButtonLeft   = 1
	mouseButtonRight  = 3
	mouseButtonMiddle = 2
	mouseButtonBack   = 7
)

func linuxFocusedApplicationIdentity() (string, int) {
	if os.Getenv("DISPLAY") == "" {
		return "", 0
	}

	display := C.neru_ax_open_display()
	if display == nil {
		return "", 0
	}
	defer C.neru_ax_close_display(display)

	var window C.Window
	if C.neru_ax_get_active_window(display, &window) == 0 {
		return "", 0
	}

	className := C.neru_ax_window_class(display, window)

	bundleID := ""
	if className != nil {
		bundleID = C.GoString(className)
		C.free(unsafe.Pointer(className))
	}

	var ok C.int
	pid := int(C.neru_ax_window_pid(display, window, &ok))

	return bundleID, pid
}

func linuxApplicationBundleIdentifier(pid int) string {
	if pid <= 0 {
		return ""
	}

	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}

	return filepath.Base(parts[0])
}

func x11MoveMouseToPoint(point image.Point) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move X11 pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

func x11CurrentCursorPosition() image.Point {
	display, err := x11ActionDisplay()
	if err != nil {
		return image.Point{}
	}
	defer C.neru_ax_close_display(display)

	var x, y C.int
	if C.neru_ax_query_pointer(display, &x, &y) == 0 {
		return image.Point{}
	}

	return image.Point{X: int(x), Y: int(y)}
}

func x11LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	return x11ClickButtonAtPoint(point, restoreCursor, modifiers, mouseButtonLeft)
}

func x11RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	return x11ClickButtonAtPoint(point, restoreCursor, modifiers, mouseButtonRight)
}

func x11MiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return x11ClickButtonAtPoint(point, restoreCursor, modifiers, mouseButtonMiddle)
}

// x11Button maps a domain mouse button to its X11 button number.
func x11Button(button action.MouseButton) C.uint {
	switch button {
	case action.ButtonRight:
		return mouseButtonRight
	case action.ButtonMiddle:
		return mouseButtonMiddle
	case action.ButtonLeft:
		fallthrough
	default:
		return mouseButtonLeft
	}
}

func x11MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	return x11MouseButtonAtPoint(point, modifiers, x11Button(button), true, false)
}

func x11MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	return x11MouseButtonAtPoint(point, modifiers, x11Button(button), false, false)
}

// x11MouseUp releases button at the cursor, then releases modifiers — the set
// its press is holding, which nothing else will undo.
func x11MouseUp(button action.MouseButton, modifiers action.Modifiers) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	// Runs before the display is closed: defers unwind last-in first-out.
	defer x11ReleaseModifiers(display, modifiers)

	if C.neru_ax_button(display, x11Button(button), 0) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to release %s mouse button on X11",
			button,
		)
	}

	return nil
}

func x11ScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	// Held across every scroll button click, the way a person holding ctrl and
	// turning the wheel produces a zoom. Runs before the display is closed:
	// defers unwind last-in first-out.
	//
	// These are the same unconditional helpers the click paths use, so the
	// release lets go of the whole named set — including a key the user is
	// physically holding, or one a sticky modifier is holding through XTest.
	// That gap predates this caller and is not narrowed here.
	x11PressModifiers(display, modifiers)
	defer x11ReleaseModifiers(display, modifiers)

	// X11 scrolling is simulated via discrete button clicks (4, 5, 6, 7).
	// Incoming deltas are pixel-level values from the scroll service config
	// (e.g. ScrollStep=50, ScrollStepHalf=500, ScrollStepFull=1000000).
	// We scale them to a capped number of clicks to avoid flooding X11
	// with tens of thousands of button events on large scrolls.
	const (
		scale     = 30
		maxClicks = 50
	)

	if deltaY != 0 {
		yClicks := abs(deltaY) / scale
		if yClicks == 0 {
			yClicks = 1
		}

		if yClicks > maxClicks {
			yClicks = maxClicks
		}

		for range yClicks {
			const mouseButtonVerticalScroll = 4
			button := C.uint(mouseButtonVerticalScroll)

			if deltaY < 0 {
				button = 5
			}

			if C.neru_ax_button(display, button, 1) == 0 ||
				C.neru_ax_button(display, button, 0) == 0 {
				return derrors.New(derrors.CodeActionFailed, "failed vertical scroll event on X11")
			}
		}
	}

	if deltaX != 0 {
		xClicks := abs(deltaX) / scale
		if xClicks == 0 {
			xClicks = 1
		}

		if xClicks > maxClicks {
			xClicks = maxClicks
		}

		for range xClicks {
			const mouseButtonHorizontalScrollRight = 7
			button := C.uint(mouseButtonHorizontalScrollRight)

			if deltaX < 0 {
				button = 6
			}

			if C.neru_ax_button(display, button, 1) == 0 ||
				C.neru_ax_button(display, button, 0) == 0 {
				return derrors.New(
					derrors.CodeActionFailed,
					"failed horizontal scroll event on X11",
				)
			}
		}
	}

	return nil
}

func x11ClickButtonAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
	button C.uint,
) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}

	defer C.neru_ax_close_display(display)

	original := x11CurrentCursorPosition()
	x11PressModifiers(display, modifiers)
	defer x11ReleaseModifiers(display, modifiers)

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move X11 pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	if C.neru_ax_button(display, button, 1) == 0 ||
		C.neru_ax_button(display, button, 0) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to dispatch X11 button click",
		)
	}

	if restoreCursor {
		_ = C.neru_ax_move_pointer(display, C.int(original.X), C.int(original.Y))
	}

	return nil
}

func x11MouseButtonAtPoint(
	point image.Point,
	modifiers action.Modifiers,
	button C.uint,
	isDown bool,
	restoreCursor bool,
) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	original := x11CurrentCursorPosition()
	x11PressModifiers(display, modifiers)

	// Only release modifiers within this function for mouse-up events.
	// For mouse-down (isDown=true), modifiers must stay held until the
	// corresponding mouse-up call; releasing them here would break
	// modifier+drag operations (e.g. Shift+drag).
	if !isDown {
		defer x11ReleaseModifiers(display, modifiers)
	}

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 {
		// If we failed to move and modifiers are held for a mouse-down,
		// release them now to avoid stuck modifier keys.
		if isDown {
			x11ReleaseModifiers(display, modifiers)
		}

		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move X11 pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	pressed := 0
	if isDown {
		pressed = 1
	}

	if C.neru_ax_button(display, button, C.int(pressed)) == 0 {
		// Release modifiers on failure to avoid stuck keys.
		if isDown {
			x11ReleaseModifiers(display, modifiers)
		}

		return derrors.New(
			derrors.CodeActionFailed,
			"failed to dispatch X11 mouse button event",
		)
	}

	if restoreCursor {
		_ = C.neru_ax_move_pointer(display, C.int(original.X), C.int(original.Y))
	}

	return nil
}

func x11ActionDisplay() (*C.Display, error) {
	if os.Getenv("DISPLAY") == "" {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"DISPLAY is not set; X11 action backend is unavailable",
		)
	}

	display := C.neru_ax_open_display()
	if display == nil {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"failed to open X11 display for mouse injection",
		)
	}

	return display, nil
}

func x11PressModifiers(display *C.Display, modifiers action.Modifiers) {
	if modifiers.Has(action.ModShift) {
		C.neru_ax_press_modifier(display, C.XK_Shift_L)
	}
	if modifiers.Has(action.ModCtrl) {
		C.neru_ax_press_modifier(display, C.XK_Control_L)
	}
	if modifiers.Has(action.ModAlt) {
		C.neru_ax_press_modifier(display, C.XK_Alt_L)
	}
	if modifiers.Has(action.ModCmd) {
		C.neru_ax_press_modifier(display, C.XK_Super_L)
	}
}

func x11ReleaseModifiers(display *C.Display, modifiers action.Modifiers) {
	if modifiers.Has(action.ModCmd) {
		C.neru_ax_release_modifier(display, C.XK_Super_L)
	}
	if modifiers.Has(action.ModAlt) {
		C.neru_ax_release_modifier(display, C.XK_Alt_L)
	}
	if modifiers.Has(action.ModCtrl) {
		C.neru_ax_release_modifier(display, C.XK_Control_L)
	}
	if modifiers.Has(action.ModShift) {
		C.neru_ax_release_modifier(display, C.XK_Shift_L)
	}
}
