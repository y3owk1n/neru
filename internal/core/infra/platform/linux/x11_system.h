#ifndef X11_SYSTEM_H
#define X11_SYSTEM_H

#include <X11/Xlib.h>
#include <X11/Xutil.h>

typedef struct {
	int x;
	int y;
	int width;
	int height;
	int primary;
	char *name;
} NeruX11Monitor;

Display *neru_x11_open_display(void);
void neru_x11_close_display(Display *display);
int neru_x11_query_pointer(Display *display, int *x, int *y);
int neru_x11_move_pointer(Display *display, int x, int y);
int neru_x11_get_active_window(Display *display, Window *out);
unsigned long neru_x11_get_window_pid(Display *display, Window window, int *ok);
NeruX11Monitor *neru_x11_get_monitors(Display *display, int *count);
void neru_x11_free_monitors(NeruX11Monitor *monitors, int count);

#endif /* X11_SYSTEM_H */
