#ifndef X11_HOTKEYS_H
#define X11_HOTKEYS_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>

Window neru_hotkeys_root_window(Display *display);
int neru_hotkeys_pending(Display *display);
int neru_xevent_type(XEvent *ev);
unsigned int neru_xkey_keycode(XEvent *ev);
unsigned int neru_xkey_state(XEvent *ev);

#endif /* X11_HOTKEYS_H */
