//go:build linux && cgo

package eventtap

/*
#cgo linux pkg-config: x11
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <stdlib.h>

static Display* neru_eventtap_open(void) {
	return XOpenDisplay(NULL);
}

static void neru_eventtap_close(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static int neru_eventtap_grab_keyboard(Display *display) {
	return XGrabKeyboard(
		display,
		DefaultRootWindow(display),
		True,
		GrabModeAsync, // keyboard_mode
		GrabModeAsync, // pointer_mode
		CurrentTime
	);
}

static void neru_eventtap_ungrab_keyboard(Display *display) {
	XUngrabKeyboard(display, CurrentTime);
	XFlush(display);
}

static int neru_eventtap_pending(Display *display) {
	return XPending(display);
}

static int neru_eventtap_next(Display *display, XEvent *event) {
	XNextEvent(display, event);
	return event->type;
}
*/
import "C"

import (
	"os"
	"strings"
	"time"
	"unsafe"
)

const (
	x11PollingInterval = 10 * time.Millisecond
	x11KeyBufferSize   = 64
)

func (et *EventTap) runX11() {
	defer close(et.doneCh)

	// Do not attempt an X11 keyboard grab if we are running under Wayland.
	// XWayland grabs conflict with Wayland compositor focus policies and frequently
	// result in the compositor sending synthetic "Escape" or "Cancel" keycodes
	// to forcefully break the unauthorized grab, accidentally quitting modes.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}

	if os.Getenv("DISPLAY") == "" {
		return
	}

	display := C.neru_eventtap_open()
	if display == nil {
		return
	}
	defer C.neru_eventtap_close(display) //nolint:nlreturn

	if C.neru_eventtap_grab_keyboard(display) != C.GrabSuccess { //nolint:nlreturn
		return
	}
	defer C.neru_eventtap_ungrab_keyboard(display) //nolint:nlreturn

	for {
		select {
		case <-et.stopCh:
			return
		default:
		}

		if C.neru_eventtap_pending(display) == 0 { //nolint:nlreturn
			time.Sleep(x11PollingInterval)

			continue
		}

		var event C.XEvent
		if C.neru_eventtap_next(display, &event) != C.KeyPress { //nolint:nlreturn
			continue
		}

		xkey := (*C.XKeyEvent)(unsafe.Pointer(&event))
		buffer := make([]C.char, x11KeyBufferSize)
		var keysym C.KeySym
		length := C.XLookupString(
			xkey,
			&buffer[0],
			C.int(len(buffer)),
			&keysym,
			nil, //nolint:nlreturn
		)

		var key string
		if length > 0 {
			key = C.GoStringN(&buffer[0], length)
		} else {
			key = x11KeysymName(keysym)
		}
		key = normalizeLinuxKey(key)
		if strings.HasPrefix(key, "__modifier_") && !et.stickyToggleEnabled() {
			continue
		}

		if key != "" {
			et.dispatchKey(key)
		}
	}
}

func x11KeysymName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Return:
		return "Return"
	case C.XK_space:
		return "Space"
	case C.XK_Tab:
		return "Tab"
	case C.XK_Escape:
		return "Escape"
	case C.XK_BackSpace:
		return "Backspace"
	case C.XK_Left:
		return "Left"
	case C.XK_Right:
		return "Right"
	case C.XK_Up:
		return "Up"
	case C.XK_Down:
		return "Down"
	case C.XK_Shift_L, C.XK_Shift_R:
		return "__modifier_shift"
	case C.XK_Control_L, C.XK_Control_R:
		return "__modifier_ctrl"
	case C.XK_Alt_L, C.XK_Alt_R:
		return "__modifier_alt"
	case C.XK_Super_L, C.XK_Super_R, C.XK_Meta_L, C.XK_Meta_R:
		return "__modifier_cmd"
	default:
		return ""
	}
}
