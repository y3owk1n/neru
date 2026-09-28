#ifndef X11_EVENTTAP_H
#define X11_EVENTTAP_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>

Display *neru_eventtap_open(void);
void neru_eventtap_close(Display *display);
int neru_eventtap_set_detectable_autorepeat(Display *display);
int neru_eventtap_grab_keyboard(Display *display);
void neru_eventtap_ungrab_keyboard(Display *display);
int neru_eventtap_pending(Display *display);
int neru_eventtap_next(Display *display, XEvent *event);
int neru_eventtap_post_modifier(const char *modifier, int is_down);

// The ASCII-capable layouts of the server's keymap, as a mask with bit g set
// for layout g. x11ReferenceGroup in the tap decides what to do with it.
unsigned neru_eventtap_ascii_groups(Display *display);

// Copies the XKB name of each layout of the server's keymap into names, at
// most max of them, and returns how many it wrote. The caller frees each name
// with free. A layout the keymap leaves unnamed gets "".
int neru_eventtap_layout_names(Display *display, char **names, int max);

// The layout the keyboard is in now, or -1 when the server does not say.
int neru_eventtap_active_group(Display *display);

#endif /* X11_EVENTTAP_H */
