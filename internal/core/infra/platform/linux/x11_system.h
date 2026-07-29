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
// neru_x11_get_window_class returns a heap-allocated copy of the window's
// WM_CLASS "class" field (res_class), or NULL when unavailable. The caller
// owns the returned pointer and must free() it.
char *neru_x11_get_window_class(Display *display, Window window);
NeruX11Monitor *neru_x11_get_monitors(Display *display, int *count);
void neru_x11_free_monitors(NeruX11Monitor *monitors, int count);

// NeruX11FocusMonitor watches _NET_ACTIVE_WINDOW on the root window from a
// dedicated X11 connection and thread, pushing a byte down a self-pipe whenever
// the active window changes so callers can wake on focus changes rather than
// polling. Opaque to Go; the fields live in x11_system.c.
typedef struct NeruX11FocusMonitor NeruX11FocusMonitor;

// neru_x11_focus_monitor_start opens a dedicated display, subscribes to
// _NET_ACTIVE_WINDOW property changes on the root window, and spawns a thread
// that signals its self-pipe on each change. Returns NULL when DISPLAY is unset
// or X11 is otherwise unavailable. The returned monitor is owned by the caller
// and released with neru_x11_focus_monitor_stop.
NeruX11FocusMonitor *neru_x11_focus_monitor_start(void);

// neru_x11_focus_monitor_fd returns a readable fd that becomes readable on
// active-window changes, or -1. Owned by the monitor; callers must not close it.
int neru_x11_focus_monitor_fd(NeruX11FocusMonitor *monitor);

// neru_x11_focus_monitor_stop signals the thread to exit, joins it, and frees
// the monitor. Safe to call with NULL.
void neru_x11_focus_monitor_stop(NeruX11FocusMonitor *monitor);

#endif /* X11_SYSTEM_H */
