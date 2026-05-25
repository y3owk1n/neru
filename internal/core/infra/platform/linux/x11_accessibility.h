#ifndef X11_ACCESSIBILITY_H
#define X11_ACCESSIBILITY_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>

Display* neru_ax_open_display(void);
void neru_ax_close_display(Display *display);
int neru_ax_query_pointer(Display *display, int *x, int *y);
int neru_ax_get_active_window(Display *display, Window *out);
unsigned long neru_ax_window_pid(Display *display, Window window, int *ok);
char* neru_ax_window_class(Display *display, Window window);
int neru_ax_move_pointer(Display *display, int x, int y);
int neru_ax_button(Display *display, unsigned int button, int pressed);
void neru_ax_press_modifier(Display *display, KeySym keysym);
void neru_ax_release_modifier(Display *display, KeySym keysym);

#endif /* X11_ACCESSIBILITY_H */
