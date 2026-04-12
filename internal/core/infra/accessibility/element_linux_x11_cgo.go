//go:build linux && cgo

package accessibility

/*
#cgo linux pkg-config: x11 xtst
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <string.h>

static Display* neru_ax_open_display(void) {
	return XOpenDisplay(NULL);
}

static void neru_ax_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static Window neru_ax_root_window(Display *display) {
	return RootWindow(display, DefaultScreen(display));
}

static int neru_ax_query_pointer(Display *display, int *x, int *y) {
	Window root_return;
	Window child_return;
	int win_x, win_y;
	unsigned int mask_return;

	return XQueryPointer(
		display,
		neru_ax_root_window(display),
		&root_return,
		&child_return,
		x,
		y,
		&win_x,
		&win_y,
		&mask_return
	);
}

static int neru_ax_get_active_window(Display *display, Window *out) {
	Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	int status = XGetWindowProperty(
		display,
		neru_ax_root_window(display),
		property,
		0,
		1,
		False,
		XA_WINDOW,
		&actual_type,
		&actual_format,
		&item_count,
		&bytes_after,
		&data
	);

	if (status != Success || data == NULL || item_count == 0) {
		if (data != NULL) {
			XFree(data);
		}
		return 0;
	}

	*out = *((Window *)data);
	XFree(data);

	if (*out == 0) {
		return 0;
	}

	return 1;
}

static unsigned long neru_ax_window_pid(Display *display, Window window, int *ok) {
	if (window == 0) {
		*ok = 0;
		return 0;
	}

	Atom property = XInternAtom(display, "_NET_WM_PID", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	int status = XGetWindowProperty(
		display,
		window,
		property,
		0,
		1,
		False,
		XA_CARDINAL,
		&actual_type,
		&actual_format,
		&item_count,
		&bytes_after,
		&data
	);

	if (status != Success || data == NULL || item_count == 0) {
		if (data != NULL) {
			XFree(data);
		}
		*ok = 0;
		return 0;
	}

	*ok = 1;
	unsigned long pid = *((unsigned long *)data);
	XFree(data);
	return pid;
}

static char* neru_ax_window_class(Display *display, Window window) {
	XClassHint hint;
	if (XGetClassHint(display, window, &hint) == 0) {
		return NULL;
	}

	char *class_name = NULL;
	if (hint.res_class != NULL) {
		class_name = strdup(hint.res_class);
	}

	if (hint.res_name != NULL) {
		XFree(hint.res_name);
	}
	if (hint.res_class != NULL) {
		XFree(hint.res_class);
	}

	return class_name;
}

static int neru_ax_move_pointer(Display *display, int x, int y) {
	int ok = XTestFakeMotionEvent(display, -1, x, y, CurrentTime);
	XFlush(display);
	return ok;
}

static int neru_ax_button(Display *display, unsigned int button, int pressed) {
	int ok = XTestFakeButtonEvent(display, button, pressed ? True : False, CurrentTime);
	XFlush(display);
	return ok;
}

static void neru_ax_press_modifier(Display *display, KeySym keysym) {
	KeyCode keycode = XKeysymToKeycode(display, keysym);
	if (keycode != 0) {
		XTestFakeKeyEvent(display, keycode, True, CurrentTime);
		XFlush(display);
	}
}

static void neru_ax_release_modifier(Display *display, KeySym keysym) {
	KeyCode keycode = XKeysymToKeycode(display, keysym);
	if (keycode != 0) {
		XTestFakeKeyEvent(display, keycode, False, CurrentTime);
		XFlush(display);
	}
}
*/
import "C"

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
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
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	var window C.Window
	if C.neru_ax_get_active_window(display, &window) == 0 { //nolint:nlreturn
		return "", 0
	}

	className := C.neru_ax_window_class(display, window) //nolint:nlreturn

	bundleID := ""
	if className != nil {
		bundleID = C.GoString(className)
		C.free(unsafe.Pointer(className))
	}

	var ok C.int
	pid := int(C.neru_ax_window_pid(display, window, &ok)) //nolint:nlreturn

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
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
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
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	var x, y C.int
	if C.neru_ax_query_pointer(display, &x, &y) == 0 { //nolint:nlreturn
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

func x11LeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	return x11MouseButtonAtPoint(point, modifiers, 1, true, false)
}

func x11LeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	return x11MouseButtonAtPoint(point, modifiers, 1, false, false)
}

func x11LeftMouseUp() error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	if C.neru_ax_button(display, 1, 0) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to release left mouse button on X11",
		)
	}

	return nil
}

func x11ScrollAtCursor(deltaX, deltaY int) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	// X11 scrolling is simulated via discrete button clicks (4, 5, 6, 7).
	// Incoming deltas are pixel-level values from the scroll service config
	// (e.g. ScrollStep=50, ScrollStepHalf=500, ScrollStepFull=1000000).
	// We scale them to a reasonable number of discrete clicks and cap the
	// maximum to avoid flooding X11 with tens of thousands of button events
	// when ScrollStepFull (scroll-to-end) sends very large deltas.
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

			if C.neru_ax_button(display, button, 1) == 0 || //nolint:nlreturn
				C.neru_ax_button(display, button, 0) == 0 { //nolint:nlreturn
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

			if C.neru_ax_button(display, button, 1) == 0 || //nolint:nlreturn
				C.neru_ax_button(display, button, 0) == 0 { //nolint:nlreturn

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

	defer C.neru_ax_close_display(display) //nolint:nlreturn

	original := x11CurrentCursorPosition()
	x11PressModifiers(display, modifiers)
	defer x11ReleaseModifiers(display, modifiers)

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move X11 pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	if C.neru_ax_button(display, button, 1) == 0 || //nolint:nlreturn
		C.neru_ax_button(display, button, 0) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to dispatch X11 button click",
		)
	}

	if restoreCursor {
		_ = C.neru_ax_move_pointer(display, C.int(original.X), C.int(original.Y)) //nolint:nlreturn
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
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	original := x11CurrentCursorPosition()
	x11PressModifiers(display, modifiers)

	// Only release modifiers within this function for mouse-up events.
	// For mouse-down (isDown=true), modifiers must stay held until the
	// corresponding mouse-up call; releasing them here would break
	// modifier+drag operations (e.g. Shift+drag).
	if !isDown {
		defer x11ReleaseModifiers(display, modifiers)
	}

	if C.neru_ax_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
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

	if C.neru_ax_button(display, button, C.int(pressed)) == 0 { //nolint:nlreturn
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
		_ = C.neru_ax_move_pointer(display, C.int(original.X), C.int(original.Y)) //nolint:nlreturn
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
		C.neru_ax_press_modifier(display, C.XK_Meta_L)
	}
}

func x11ReleaseModifiers(display *C.Display, modifiers action.Modifiers) {
	if modifiers.Has(action.ModCmd) {
		C.neru_ax_release_modifier(display, C.XK_Meta_L)
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

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
