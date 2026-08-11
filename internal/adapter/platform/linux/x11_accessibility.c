#include "x11_accessibility.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <string.h>

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

int neru_ax_get_active_window(Display *display, Window *out) {
	Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	int status = XGetWindowProperty(
	    display, neru_ax_root_window(display), property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format,
	    &item_count, &bytes_after, &data);

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

unsigned long neru_ax_window_pid(Display *display, Window window, int *ok) {
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
	    display, window, property, 0, 1, False, XA_CARDINAL, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);

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

char *neru_ax_window_class(Display *display, Window window) {
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

void neru_ax_press_modifier(Display *display, KeySym keysym) {
	KeyCode keycode = XKeysymToKeycode(display, keysym);
	if (keycode != 0) {
		XTestFakeKeyEvent(display, keycode, True, CurrentTime);
		XFlush(display);
	}
}

void neru_ax_release_modifier(Display *display, KeySym keysym) {
	KeyCode keycode = XKeysymToKeycode(display, keysym);
	if (keycode != 0) {
		XTestFakeKeyEvent(display, keycode, False, CurrentTime);
		XFlush(display);
	}
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
