#include "x11_hotkeys.h"

#include <X11/XKBlib.h>
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdlib.h>

Window neru_hotkeys_root_window(Display *display) { return RootWindow(display, DefaultScreen(display)); }

// Asks the server to report a held key as repeated KeyPress events rather than
// as release/press pairs. Returns 1 when the server honors it for this
// connection, 0 otherwise.
int neru_hotkeys_set_detectable_autorepeat(Display *display) {
	Bool supported = False;
	XkbSetDetectableAutoRepeat(display, True, &supported);
	return supported ? 1 : 0;
}

int neru_hotkeys_pending(Display *display) { return XPending(display); }

int neru_xevent_type(XEvent *ev) { return ev->type; }

unsigned int neru_xkey_keycode(XEvent *ev) { return ev->xkey.keycode; }

unsigned int neru_xkey_state(XEvent *ev) { return ev->xkey.state; }
