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
// X11 window, as a WM_CLASS/pid pair, and ("", 0) when it cannot say.
//
// The active window and the identity of whatever it names come from the system
// bridge (x11_system.h) rather than from a second copy in this one. They were
// two copies until #1499: the system one had learned to tell a window manager
// with nothing focused from a display owned by none, and to survive a property
// naming a window that has since closed, while the copy here still let that
// close exit the daemon.
//
// The five answers that bridge distinguishes collapse to one here on purpose.
// This signature has no error to carry, and its callers — focused-app identity
// and the frontmost-window queries behind it — re-sample on the next focus
// event either way, so "no identity" is the whole of what they can act on.
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

	// The pid answers collapse here the same way the active-window ones do: this
	// signature has no error to carry, and a window that advertises no pid and
	// one that closed under the query both leave this caller with no pid to
	// report. system_x11_window_pid.go is where the difference is told, for the
	// caller that can act on it.
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

// x11MouseUp releases button at the cursor and undoes the hold its press took —
// the modifiers that press is presenting, which nothing else will undo.
//
// modifiers is what the caller believes the press is holding, and is used only
// when this connection finds no press to pick up from.
func x11MouseUp(button action.MouseButton, modifiers action.Modifiers) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display)

	// Taken before the button event so the release still goes out under the
	// modifiers the press presented, and undone after it. Runs before the
	// display is closed: defers unwind last-in first-out.
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

	// Held across every scroll button click, the way a person holding ctrl and
	// turning the wheel produces a zoom — and, just as much, *not* held when
	// nothing asked for it: an X11 button event carries whatever the server
	// records as down, so a plain scroll_down bound to Ctrl+J would zoom. The
	// hold presents exactly this set for the length of the whole loop. Runs
	// before the display is closed: defers unwind last-in first-out.
	hold := x11HoldModifiers(display, modifiers)
	defer hold.release()

	// X11 scrolling is simulated via discrete button clicks (4, 5, 6, 7).
	// Incoming deltas are pixel-level values from the scroll service config
	// (e.g. ScrollStep=50, ScrollStepHalf=500, ScrollStepFull=1000000).
	// We scale them to a capped number of clicks to avoid flooding X11
	// with tens of thousands of button events on large scrolls.
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
// holding the display connection and any modifiers for the length of the
// animation.
//
// X11 has no sub-notch scroll to synthesize. Core scrolling is buttons 4 to 7
// and a button event is one notch by definition, and the XTEST pointer the
// server creates for XTestFakeButtonEvent has no scroll valuators at all — it
// is allocated with two axes, Rel X and Rel Y — so the smooth-scrolling XI2
// path real devices use is not reachable from here. What an animation can still
// do is spread those notches over time on the same eased curve every other
// backend uses, which is why granularity is a whole notch rather than zero.
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

	// Taken once for the whole animation rather than per chunk, so every chunk
	// goes out under the same modifier state: the requested set presented, and
	// anything the user happens to be holding suppressed for the duration.
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

// clickAxis emits one chunk as whole wheel clicks. The animator only ever hands
// it exact multiples of a notch, so the rounding here settles floating-point
// error rather than a real fraction.
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

	// An X11 button event carries whatever the server records as down rather
	// than a set the sender chooses, so the click presents exactly this set:
	// a modifier the user's hand is on is suppressed for the length of the
	// click, and one they are holding is left held afterwards. Runs before the
	// display is closed: defers unwind last-in first-out.
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

	// A press takes the hold; a release picks up the one its press took, so
	// both events and the drag between them present the same modifiers.
	//
	// A press keeps its hold until that release — undoing it here would make a
	// modified drag unmodified from its first pixel of movement — so only a
	// release undoes it on the way out. That defer runs before the display is
	// closed: defers unwind last-in first-out.
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
		// Same as above: no release is coming, so undo the hold here.
		if isDown {
			hold.release()
		}

		return derrors.New(
			derrors.CodeActionFailed,
			"failed to dispatch X11 mouse button event",
		)
	}

	// The press succeeded, so its hold outlives this call and this connection:
	// the release picks it up from here.
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
