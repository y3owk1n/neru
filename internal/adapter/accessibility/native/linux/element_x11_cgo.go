//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <stdlib.h>
#include "../../../platform/linux/x11_accessibility.h"
#include "../../../platform/linux/x11_system.h"
*/
import "C"

import (
	"image"
	"math"
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

// linuxFocusedApplicationIdentity answers which application owns the focused
// X11 window, as a WM_CLASS/pid pair, and ("", 0) when it cannot say. The
// active window and its identity come from the system bridge (x11_system.h),
// not a second copy here (#1499), since that copy let a closed window exit the
// daemon. The bridge's five answers collapse to one on purpose. This signature
// has no error, and its callers re-sample on the next focus event anyway.
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
	if C.neru_x11_get_active_window(display, &window) != C.int(C.NERU_X11_ACTIVE_WINDOW_OK) {
		return "", 0
	}

	className := C.neru_x11_get_window_class(display, window)

	bundleID := ""
	if className != nil {
		bundleID = C.GoString(className)
		C.free(unsafe.Pointer(className))
	}

	// The pid answers collapse the same way, since this caller has no error to
	// carry. system_x11_window_pid.go tells them apart for the caller that can
	// act on it.
	var pid C.ulong
	if C.neru_x11_get_window_pid(display, window, &pid) != C.int(C.NERU_X11_WINDOW_PID_OK) {
		pid = 0
	}

	return bundleID, int(pid)
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

// x11MouseUp releases button at the cursor and undoes the modifier hold its
// press took, which nothing else will. modifiers is used only when this
// connection finds no press to pick up from.
func x11MouseUp(button action.MouseButton, modifiers action.Modifiers) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	// Taken before the button event and undone after it, before the display
	// closes (defers run last-in first-out).
	hold := x11ResumeModifierHold(display, x11Button(button), modifiers)
	defer hold.release()

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

	// Held across every click, as ctrl plus a wheel turn zooms, and present only
	// when asked, since a plain scroll_down bound to Ctrl+J would otherwise zoom.
	// Undone before the display closes (defers run last-in first-out).
	hold := x11HoldModifiers(display, modifiers)
	defer hold.release()

	// X11 scrolls with button clicks (4 to 7). Pixel deltas from the scroll
	// config (ScrollStepFull=1000000) are scaled to a capped click count so a
	// large scroll does not flood X11.
	const maxClicks = maxScrollUnitsPerRequest

	if deltaY != 0 {
		yClicks := min(scrollNotches(deltaY), maxClicks)

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
		xClicks := min(scrollNotches(deltaX), maxClicks)

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

// X11 scroll buttons, in the order a wheel reports them.
const (
	x11ScrollUp    = C.uint(4)
	x11ScrollDown  = C.uint(5)
	x11ScrollLeft  = C.uint(6)
	x11ScrollRight = C.uint(7)
)

// x11ScrollSession injects an animated scroll on X11 as wheel-button clicks,
// holding the display connection and any modifiers for the animation. X11 has
// no sub-notch scroll to synthesize. A core scroll button event is one notch,
// and the XTEST pointer has only Rel X and Rel Y axes, so the XI2 smooth path
// is unreachable. The animation spreads whole notches over the eased curve,
// hence a granularity of one notch.
type x11ScrollSession struct {
	display *C.Display
	hold    x11ModifierHold
}

// x11ScrollBackendAvailable answers whether an animated scroll could inject
// here, without opening a connection to find out.
func x11ScrollBackendAvailable() error {
	if os.Getenv("DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"DISPLAY is not set; X11 action backend is unavailable",
		)
	}

	return nil
}

func newX11ScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	display, err := x11ActionDisplay()
	if err != nil {
		return nil, err
	}

	// Taken once for the whole animation so every chunk presents the requested
	// set, with anything the user holds suppressed throughout.
	return &x11ScrollSession{display: display, hold: x11HoldModifiers(display, modifiers)}, nil
}

func (s *x11ScrollSession) granularity() float64 { return scrollPixelsPerNotch }

func (s *x11ScrollSession) inject(deltaX, deltaY float64) error {
	err := s.clickAxis(deltaY, x11ScrollUp, x11ScrollDown)
	if err != nil {
		return err
	}

	return s.clickAxis(deltaX, x11ScrollRight, x11ScrollLeft)
}

// clickAxis emits one chunk as whole wheel clicks. The animator hands it exact
// notch multiples, so rounding only settles floating-point error.
func (s *x11ScrollSession) clickAxis(delta float64, positive, negative C.uint) error {
	if delta == 0 {
		return nil
	}

	button := positive
	if delta < 0 {
		button = negative
	}

	clicks := int(math.Round(math.Abs(delta) / scrollPixelsPerNotch))

	for range clicks {
		if C.neru_ax_button(s.display, button, 1) == 0 ||
			C.neru_ax_button(s.display, button, 0) == 0 {
			return derrors.New(derrors.CodeActionFailed, "failed scroll event on X11")
		}
	}

	return nil
}

func (s *x11ScrollSession) close() {
	s.hold.release()
	C.neru_ax_close_display(s.display)
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

	// The click presents exactly this set, suppressing a held modifier only for
	// the click (see x11HoldModifiers). Undone before the display closes, since
	// defers run last-in first-out.
	hold := x11HoldModifiers(display, modifiers)
	defer hold.release()

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

	// A press takes the hold and its release picks it up, so both events and the
	// drag between present the same modifiers. Only a release undoes it, since
	// undoing at the press would unmodify the drag. That defer runs before the
	// display closes (defers run last-in first-out).
	var hold x11ModifierHold

	if isDown {
		hold = x11HoldModifiers(display, modifiers)
	} else {
		hold = x11ResumeModifierHold(display, button, modifiers)

		defer hold.release()
	}

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 {
		// Nothing will come to release a press that never happened, so a
		// failure has to undo its own hold rather than leave keys stuck.
		if isDown {
			hold.release()
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
		// Same as above. No release is coming, so undo the hold here.
		if isDown {
			hold.release()
		}

		return derrors.New(
			derrors.CodeActionFailed,
			"failed to dispatch X11 mouse button event",
		)
	}

	// The press succeeded, so its hold outlives this call and this connection.
	// The release picks it up from here.
	if isDown {
		hold.keepForRelease(button)
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
