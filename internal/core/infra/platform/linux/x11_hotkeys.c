#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include "x11_hotkeys.h"


Window neru_hotkeys_root_window(Display *display) {
	return RootWindow(display, DefaultScreen(display));
}

int neru_hotkeys_pending(Display *display) {
	return XPending(display);
}

int neru_xevent_type(XEvent *ev) {
	return ev->type;
}

unsigned int neru_xkey_keycode(XEvent *ev) {
	return ev->xkey.keycode;
}

unsigned int neru_xkey_state(XEvent *ev) {
	return ev->xkey.state;
}
