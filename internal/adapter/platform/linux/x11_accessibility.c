#include "x11_accessibility.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/keysym.h>
#include <stdlib.h>

Display *neru_ax_open_display(void) { return XOpenDisplay(NULL); }

void neru_ax_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static Window neru_ax_root_window(Display *display) { return RootWindow(display, DefaultScreen(display)); }

int neru_ax_query_pointer(Display *display, int *x, int *y) {
	Window root_return;
	Window child_return;
	int win_x, win_y;
	unsigned int mask_return;

	return XQueryPointer(
	    display, neru_ax_root_window(display), &root_return, &child_return, x, y, &win_x, &win_y, &mask_return);
}

int neru_ax_move_pointer(Display *display, int x, int y) {
	int ok = XTestFakeMotionEvent(display, -1, x, y, CurrentTime);
	XFlush(display);
	return ok;
}

int neru_ax_button(Display *display, unsigned int button, int pressed) {
	int ok = XTestFakeButtonEvent(display, button, pressed ? True : False, CurrentTime);
	XFlush(display);
	return ok;
}

/* Resolves a keysym to the keycode carrying it, or 0 when the live keymap has
 * no key for it — a layout without a right Alt, say. */
unsigned int neru_ax_keysym_to_keycode(Display *display, KeySym keysym) {
	return (unsigned int)XKeysymToKeycode(display, keysym);
}

/* Reads the server's live key state into the 32-byte vector XQueryKeymap
 * defines: one bit per keycode, which is the only way to learn what the user is
 * physically holding. */
void neru_ax_query_keymap(Display *display, char *keys32) { XQueryKeymap(display, keys32); }

/* Reports whether keycode is down in a vector neru_ax_query_keymap filled. */
int neru_ax_keycode_is_held(const char *keys32, unsigned int keycode) {
	if (keycode >= NERU_AX_KEYMAP_BITS) {
		return 0;
	}

	unsigned char byte = (unsigned char)keys32[keycode / 8];

	return (byte >> (keycode % 8)) & 1;
}

/* Presses or releases a keycode through XTEST. Unlike the modifier helpers
 * above this takes the keycode the keymap answered with, so a key read as held
 * is the same key that gets released. */
void neru_ax_key_event(Display *display, unsigned int keycode, int pressed) {
	if (keycode == 0) {
		return;
	}

	XTestFakeKeyEvent(display, (KeyCode)keycode, pressed ? True : False, CurrentTime);
	XFlush(display);
}
