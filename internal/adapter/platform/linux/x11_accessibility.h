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
int neru_ax_get_active_window(Display *display, Window *out);
unsigned long neru_ax_window_pid(Display *display, Window window, int *ok);
char *neru_ax_window_class(Display *display, Window window);
int neru_ax_move_pointer(Display *display, int x, int y);
int neru_ax_button(Display *display, unsigned int button, int pressed);
void neru_ax_press_modifier(Display *display, KeySym keysym);
void neru_ax_release_modifier(Display *display, KeySym keysym);
unsigned int neru_ax_keysym_to_keycode(Display *display, KeySym keysym);
void neru_ax_query_keymap(Display *display, char *keys32);
int neru_ax_keycode_is_held(const char *keys32, unsigned int keycode);
void neru_ax_key_event(Display *display, unsigned int keycode, int pressed);

#endif /* X11_ACCESSIBILITY_H */
