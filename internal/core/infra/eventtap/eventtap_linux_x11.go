//go:build linux && cgo

package eventtap

/*
#cgo linux pkg-config: x11 xtst
#include <X11/extensions/XTest.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <string.h>

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

static KeySym neru_eventtap_modifier_keysym(const char *modifier) {
	if (strcmp(modifier, "shift") == 0) return XK_Shift_L;
	if (strcmp(modifier, "ctrl") == 0) return XK_Control_L;
	if (strcmp(modifier, "alt") == 0) return XK_Alt_L;
	if (strcmp(modifier, "cmd") == 0) return XK_Super_L;
	return NoSymbol;
}

static int neru_eventtap_post_modifier(const char *modifier, int is_down) {
	// Open a fresh Display connection per call to ensure thread-safety isolation
	// from the grab Display used by runX11(). XLib is not thread-safe without
	// XInitThreads, so we avoid sharing the grab connection for XTest injection.
	Display *display = neru_eventtap_open();
	if (display == NULL) return 0;

	KeySym keysym = neru_eventtap_modifier_keysym(modifier);
	if (keysym == NoSymbol) {
		neru_eventtap_close(display);
		return 0;
	}

	KeyCode keycode = XKeysymToKeycode(display, keysym);
	if (keycode == 0) {
		neru_eventtap_close(display);
		return 0;
	}

	int ok = XTestFakeKeyEvent(display, keycode, is_down ? True : False, CurrentTime);
	XFlush(display);
	neru_eventtap_close(display);

	return ok;
}
*/
import "C"

import (
	"os"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
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
		eventType := C.neru_eventtap_next(display, &event) //nolint:nlreturn
		if eventType != C.KeyPress && eventType != C.KeyRelease {
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

		if modifier := x11ModifierName(keysym); modifier != "" {
			isDown := eventType == C.KeyPress
			if et.consumeSyntheticModifierEvent(modifier, isDown) {
				continue
			}

			if et.stickyToggleEnabled() {
				et.dispatchKey(linuxModifierToggleEvent(modifier, isDown))
			}

			continue
		}

		if eventType != C.KeyPress {
			continue
		}

		var key string
		if length > 0 {
			key = C.GoStringN(&buffer[0], length)
		} else {
			key = x11KeysymName(keysym)
		}
		key = normalizeLinuxKey(key)

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
	default:
		return ""
	}
}

func x11ModifierName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Shift_L, C.XK_Shift_R:
		return evdevModifierShift
	case C.XK_Control_L, C.XK_Control_R:
		return evdevModifierCtrl
	case C.XK_Alt_L, C.XK_Alt_R:
		return evdevModifierAlt
	case C.XK_Super_L, C.XK_Super_R, C.XK_Meta_L, C.XK_Meta_R:
		return evdevModifierCmd
	default:
		return ""
	}
}

func postLinuxModifierEvent(modifier string, isDown bool) bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return linux.WlrootsModifierEvent(modifier, isDown) == nil
	}

	if os.Getenv("DISPLAY") == "" {
		return false
	}

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier)) //nolint:nlreturn

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	return C.neru_eventtap_post_modifier(cModifier, cDown) != 0
}
