// Test support for the X11 tap's integration tests, built only under the
// integration tag. It types through XTest by XKB key name, and acts as a
// hotkey client that holds a passive grab. Nothing here reaches a shipped
// binary.

//go:build integration && linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <X11/XKBlib.h>
#include <X11/extensions/XTest.h>
#include <stdlib.h>
#include <string.h>

static KeyCode neru_test_x11_key(Display *display, const char *name) {
	XkbDescPtr xkb = XkbGetMap(display, 0, XkbUseCoreKbd);
	if (!xkb)
		return 0;

	KeyCode found = 0;
	if (XkbGetNames(display, XkbKeyNamesMask, xkb) == Success) {
		for (int key = xkb->min_key_code; key <= xkb->max_key_code; key++) {
			if (strncmp(xkb->names->keys[key].name, name, XkbKeyNameLength) == 0)
				found = (KeyCode)key;
		}
	}

	XkbFreeKeyboard(xkb, 0, True);
	return found;
}

// neru_test_x11_press presses the named keys in order and releases them in
// reverse, so every key before the last is held while the last is typed.
// Returns 0 when there are more than eight names, a name is not in the keymap,
// or the display is missing.
static int neru_test_x11_press(const char **names, int count) {
	KeyCode keys[8] = {0};
	if (count > 8)
		return 0;

	Display *display = XOpenDisplay(NULL);
	if (!display)
		return 0;

	for (int i = 0; i < count; i++) {
		keys[i] = neru_test_x11_key(display, names[i]);
		if (keys[i] == 0) {
			XCloseDisplay(display);
			return 0;
		}
	}

	for (int i = 0; i < count; i++)
		XTestFakeKeyEvent(display, keys[i], True, CurrentTime);
	for (int i = count - 1; i >= 0; i--)
		XTestFakeKeyEvent(display, keys[i], False, CurrentTime);
	XSync(display, False);

	XCloseDisplay(display);
	return 1;
}

// neru_test_x11_hold_grabbed grabs the named key on the root window from a
// connection of its own, the way a hotkey client does, and presses it, which
// activates that grab. The key stays down until neru_test_x11_release_grabbed.
// blocked reports whether a keyboard grab from a third connection is refused
// while the key is down. The tap gets the same refusal.
static Display *neru_test_x11_hold_grabbed(const char *name, KeyCode *key, int *blocked) {
	Display *display = XOpenDisplay(NULL);
	if (!display)
		return NULL;

	*key = neru_test_x11_key(display, name);
	if (*key == 0) {
		XCloseDisplay(display);
		return NULL;
	}

	int pointer_mode = GrabModeAsync, keyboard_mode = GrabModeAsync;
	XGrabKey(display, *key, AnyModifier, DefaultRootWindow(display), True, pointer_mode, keyboard_mode);
	XTestFakeKeyEvent(display, *key, True, CurrentTime);
	XSync(display, False);

	*blocked = 0;
	Display *probe = XOpenDisplay(NULL);
	if (probe) {
		int status =
		    XGrabKeyboard(probe, DefaultRootWindow(probe), True, pointer_mode, keyboard_mode, CurrentTime);
		*blocked = status == AlreadyGrabbed;
		if (status == GrabSuccess)
			XUngrabKeyboard(probe, CurrentTime);
		XCloseDisplay(probe);
	}

	return display;
}

static void neru_test_x11_release_grabbed(Display *display, KeyCode key) {
	XTestFakeKeyEvent(display, key, False, CurrentTime);
	XUngrabKey(display, key, AnyModifier, DefaultRootWindow(display));
	XSync(display, False);
	XCloseDisplay(display);
}
// neru_test_x11_focused_window maps a window that selects key presses on a
// connection of its own and gives it the input focus, as an app the user
// types into. Returns NULL when the display is missing.
static Display *neru_test_x11_focused_window(Window *window) {
	Display *display = XOpenDisplay(NULL);
	if (!display)
		return NULL;

	*window = XCreateSimpleWindow(display, DefaultRootWindow(display), 0, 0, 64, 64, 0, 0, 0);
	XSelectInput(display, *window, KeyPressMask | StructureNotifyMask);
	XMapRaised(display, *window);

	XEvent event;
	do {
		XNextEvent(display, &event);
	} while (event.type != MapNotify);

	XSetInputFocus(display, *window, RevertToParent, CurrentTime);
	XSync(display, False);
	return display;
}

// neru_test_x11_key_presses discards the window's pending events and counts
// the key presses among them.
static int neru_test_x11_key_presses(Display *display) {
	XSync(display, False);

	int presses = 0;
	while (XPending(display)) {
		XEvent event;
		XNextEvent(display, &event);
		if (event.type == KeyPress)
			presses++;
	}

	return presses;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

var errX11PressRefused = errors.New("XTest could not press the keys")

// pressX11Keys types the named keys through XTest, holding each one until the
// last is typed.
func pressX11Keys(names ...string) error {
	cNames := make([]*C.char, len(names))
	for i, name := range names {
		cNames[i] = C.CString(name)
		defer C.free(unsafe.Pointer(cNames[i]))
	}

	if C.neru_test_x11_press(&cNames[0], C.int(len(cNames))) == 0 {
		return errX11PressRefused
	}

	return nil
}

// holdX11GrabbedKey presses the named key under a passive grab another
// connection holds, as a hotkey that starts a mode is. The returned function
// releases the key and drops the grab. The bool reports whether a keyboard grab
// is refused while the key is down.
func holdX11GrabbedKey(name string) (func(), bool, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var (
		key      C.KeyCode
		cBlocked C.int
	)

	display := C.neru_test_x11_hold_grabbed(cName, &key, &cBlocked)
	if display == nil {
		return nil, false, errX11PressRefused
	}

	return func() { C.neru_test_x11_release_grabbed(display, key) }, cBlocked != 0, nil
}

// x11FocusedWindow is a window holding the input focus, as an app the user
// types into, that counts the key presses it receives.
type x11FocusedWindow struct {
	display *C.Display
	window  C.Window
}

// openX11FocusedWindow maps a window and focuses it.
func openX11FocusedWindow() (*x11FocusedWindow, error) {
	var window C.Window

	display := C.neru_test_x11_focused_window(&window)
	if display == nil {
		return nil, errX11PressRefused
	}

	return &x11FocusedWindow{display: display, window: window}, nil
}

// keyPresses counts the key presses the window received since the last call.
func (w *x11FocusedWindow) keyPresses() int {
	return int(C.neru_test_x11_key_presses(w.display))
}

func (w *x11FocusedWindow) close() {
	C.XDestroyWindow(w.display, w.window)
	C.XCloseDisplay(w.display)
}
