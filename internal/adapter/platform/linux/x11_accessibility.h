#ifndef X11_ACCESSIBILITY_H
#define X11_ACCESSIBILITY_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>

/* NERU_AX_KEYMAP_BYTES is the size of the vector XQueryKeymap fills, and
 * NERU_AX_KEYMAP_BITS the number of keycodes it can describe. Both are fixed by
 * the core protocol rather than chosen here. */
#define NERU_AX_KEYMAP_BYTES 32
#define NERU_AX_KEYMAP_BITS (NERU_AX_KEYMAP_BYTES * 8)

Display *neru_ax_open_display(void);
void neru_ax_close_display(Display *display);
int neru_ax_query_pointer(Display *display, int *x, int *y);
/* Reading _NET_ACTIVE_WINDOW and the identity of the window it names is not
 * declared here: this bridge and the system bridge asked the same question with
 * two copies of the answer, and the copies drifted — the system one learned to
 * tell a window manager with nothing focused from a display with none, and to
 * survive a property naming a window that has closed, while this one kept
 * exiting the process on it. Callers use neru_x11_get_active_window,
 * neru_x11_get_window_pid and neru_x11_get_window_class from x11_system.h. */
int neru_ax_move_pointer(Display *display, int x, int y);
int neru_ax_button(Display *display, unsigned int button, int pressed);
unsigned int neru_ax_keysym_to_keycode(Display *display, KeySym keysym);
void neru_ax_query_keymap(Display *display, char *keys32);
int neru_ax_keycode_is_held(const char *keys32, unsigned int keycode);
void neru_ax_key_event(Display *display, unsigned int keycode, int pressed);

#endif /* X11_ACCESSIBILITY_H */
