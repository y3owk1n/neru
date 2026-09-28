#include "x11_hotkeys.h"

#include <X11/XKBlib.h>
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <strings.h>

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

KeyCode neru_hotkeys_keycode_in_layout(Display *display, KeySym keysym, const char *name) {
	XkbDescPtr xkb = XkbGetMap(display, XkbKeyTypesMask | XkbKeySymsMask, XkbUseCoreKbd);
	if (!xkb)
		return 0;

	int layout = -1;
	if (XkbGetNames(display, XkbGroupNamesMask, xkb) == Success && xkb->names) {
		for (int group = 0; group < XkbNumKbdGroups && layout < 0; group++) {
			if (xkb->names->groups[group] == None)
				break;

			char *candidate = XGetAtomName(display, xkb->names->groups[group]);
			if (candidate && strcasecmp(candidate, name) == 0)
				layout = group;
			if (candidate)
				XFree(candidate);
		}
	}

	KeyCode found = 0;
	for (int key = xkb->min_key_code; layout >= 0 && found == 0 && key <= xkb->max_key_code; key++) {
		if (layout >= XkbKeyNumGroups(xkb, key))
			continue;

		for (int level = 0; level < 2 && level < XkbKeyGroupsWidth(xkb, key); level++) {
			if (XkbKeySymEntry(xkb, key, level, layout) == keysym) {
				found = (KeyCode)key;
				break;
			}
		}
	}

	XkbFreeKeyboard(xkb, 0, True);
	return found;
}
