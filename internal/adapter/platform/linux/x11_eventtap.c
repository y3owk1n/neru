#include "x11_eventtap.h"

#include <X11/XKBlib.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/keysym.h>
#include <stdlib.h>
#include <string.h>

Display *neru_eventtap_open(void) { return XOpenDisplay(NULL); }

// A held key is otherwise reported as release/press pairs at the server's
// autorepeat rate, and the tap turned each of those releases into a key-up
// that ended the mode's own held repeat after its first tick. Detectable
// autorepeat reports the hold as further KeyPress events and one KeyRelease,
// so the key-up means the key came up. Per connection; the hotkey connection
// asks for the same (x11_hotkeys.c). Returns 1 when the server honors it.
int neru_eventtap_set_detectable_autorepeat(Display *display) {
	Bool supported = False;
	XkbSetDetectableAutoRepeat(display, True, &supported);
	return supported ? 1 : 0;
}

void neru_eventtap_close(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

int neru_eventtap_grab_keyboard(Display *display) {
	return XGrabKeyboard(
	    display, DefaultRootWindow(display), True,
	    GrabModeAsync,  // keyboard_mode
	    GrabModeAsync,  // pointer_mode
	    CurrentTime);
}

void neru_eventtap_ungrab_keyboard(Display *display) {
	XUngrabKeyboard(display, CurrentTime);
	XFlush(display);
}

int neru_eventtap_pending(Display *display) { return XPending(display); }

int neru_eventtap_next(Display *display, XEvent *event) {
	XNextEvent(display, event);
	return event->type;
}

// The top letter row, AD01 to AD10. Its symbols tell a Latin layout from any
// other without naming a layout.
static const char *const letter_row[] = {"AD01", "AD02", "AD03", "AD04", "AD05",
                                         "AD06", "AD07", "AD08", "AD09", "AD10"};

static KeyCode key_by_name(XkbDescPtr xkb, const char *name) {
	for (int key = xkb->min_key_code; key <= xkb->max_key_code; key++) {
		if (strncmp(xkb->names->keys[key].name, name, XkbKeyNameLength) == 0)
			return (KeyCode)key;
	}

	return 0;
}

// layout_is_ascii reports whether every letter-row key types a printable ASCII
// character at the base level of group.
static int layout_is_ascii(XkbDescPtr xkb, int group) {
	for (size_t i = 0; i < sizeof(letter_row) / sizeof(letter_row[0]); i++) {
		KeyCode key = key_by_name(xkb, letter_row[i]);
		if (key == 0 || group >= XkbKeyNumGroups(xkb, key))
			return 0;

		KeySym keysym = XkbKeySymEntry(xkb, key, 0, group);
		if (keysym <= 0x20 || keysym >= 0x7f)
			return 0;
	}

	return 1;
}

unsigned neru_eventtap_ascii_groups(Display *display) {
	XkbDescPtr xkb = XkbGetMap(display, XkbKeySymsMask, XkbUseCoreKbd);
	if (!xkb)
		return 0;

	unsigned groups = 0;
	if (XkbGetNames(display, XkbKeyNamesMask, xkb) == Success) {
		for (int group = 0; group < XkbNumKbdGroups; group++) {
			if (layout_is_ascii(xkb, group))
				groups |= 1u << group;
		}
	}

	XkbFreeKeyboard(xkb, 0, True);
	return groups;
}

int neru_eventtap_layout_names(Display *display, char **names, int max) {
	XkbDescPtr xkb = XkbAllocKeyboard();
	if (!xkb)
		return 0;

	int count = 0;
	if (XkbGetNames(display, XkbGroupNamesMask, xkb) == Success && xkb->names) {
		for (int group = 0; group < XkbNumKbdGroups && count < max; group++) {
			Atom atom = xkb->names->groups[group];
			if (atom == None)
				break;

			char *name = XGetAtomName(display, atom);
			names[count++] = strdup(name ? name : "");
			if (name)
				XFree(name);
		}
	}

	XkbFreeKeyboard(xkb, 0, True);
	return count;
}

int neru_eventtap_active_group(Display *display) {
	XkbStateRec state;
	if (XkbGetState(display, XkbUseCoreKbd, &state) != Success)
		return -1;

	return state.group;
}

static KeySym neru_eventtap_modifier_keysym(const char *modifier) {
	if (strcmp(modifier, "shift") == 0)
		return XK_Shift_L;
	if (strcmp(modifier, "ctrl") == 0)
		return XK_Control_L;
	if (strcmp(modifier, "alt") == 0)
		return XK_Alt_L;
	if (strcmp(modifier, "cmd") == 0)
		return XK_Super_L;
	return NoSymbol;
}

int neru_eventtap_post_modifier(const char *modifier, int is_down) {
	// Open a fresh Display connection per call to ensure thread-safety isolation
	// from the grab Display used by runX11(). XLib is not thread-safe without
	// XInitThreads, so we avoid sharing the grab connection for XTest injection.
	Display *display = neru_eventtap_open();
	if (display == NULL)
		return 0;

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
