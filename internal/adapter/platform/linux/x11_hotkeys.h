#ifndef X11_HOTKEYS_H
#define X11_HOTKEYS_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>

Window neru_hotkeys_root_window(Display *display);
int neru_hotkeys_set_detectable_autorepeat(Display *display);
int neru_hotkeys_pending(Display *display);
int neru_xevent_type(XEvent *ev);
unsigned int neru_xkey_keycode(XEvent *ev);
unsigned int neru_xkey_state(XEvent *ev);

// The keycode whose first or second level types keysym in the layout of the
// server's keymap named name, ignoring case, or 0 when the keymap has no such
// layout or no key in it types keysym.
KeyCode neru_hotkeys_keycode_in_layout(Display *display, KeySym keysym, const char *name);

#endif /* X11_HOTKEYS_H */
