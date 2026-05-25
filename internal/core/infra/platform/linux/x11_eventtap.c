#include <X11/extensions/XTest.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <string.h>
#include "x11_eventtap.h"


Display* neru_eventtap_open(void) {
	return XOpenDisplay(NULL);
}

void neru_eventtap_close(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

int neru_eventtap_grab_keyboard(Display *display) {
	return XGrabKeyboard(
		display,
		DefaultRootWindow(display),
		True,
		GrabModeAsync, // keyboard_mode
		GrabModeAsync, // pointer_mode
		CurrentTime
	);
}

void neru_eventtap_ungrab_keyboard(Display *display) {
	XUngrabKeyboard(display, CurrentTime);
	XFlush(display);
}

int neru_eventtap_pending(Display *display) {
	return XPending(display);
}

int neru_eventtap_next(Display *display, XEvent *event) {
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

int neru_eventtap_post_modifier(const char *modifier, int is_down) {
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
