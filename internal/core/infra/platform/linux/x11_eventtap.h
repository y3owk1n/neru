#ifndef X11_EVENTTAP_H
#define X11_EVENTTAP_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>

Display* neru_eventtap_open(void);
void neru_eventtap_close(Display *display);
int neru_eventtap_grab_keyboard(Display *display);
void neru_eventtap_ungrab_keyboard(Display *display);
int neru_eventtap_pending(Display *display);
int neru_eventtap_next(Display *display, XEvent *event);
int neru_eventtap_post_modifier(const char *modifier, int is_down);

#endif /* X11_EVENTTAP_H */
