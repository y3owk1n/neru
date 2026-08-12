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

// NeruX11ActiveWindowResult distinguishes the answers reading
// _NET_ACTIVE_WINDOW can give. A desktop with nothing focused answered the
// question and is not a failure — collapsing it into the failures made a user
// who clicked their wallpaper look like a broken X server. The three failures
// are kept apart from each other too, because the fix differs: start an EWMH
// window manager, look at the display server, or fix whatever wrote a
// non-conforming property.
typedef enum {
	NERU_X11_ACTIVE_WINDOW_OK = 0,            // *out holds a live window id
	NERU_X11_ACTIVE_WINDOW_NONE = 1,          // queried fine; nothing is focused
	NERU_X11_ACTIVE_WINDOW_NO_WM = 2,         // no window manager claims EWMH here
	NERU_X11_ACTIVE_WINDOW_QUERY_FAILED = 3,  // XGetWindowProperty failed
	NERU_X11_ACTIVE_WINDOW_MALFORMED = 4,     // present, but not a 32-bit WINDOW value
} NeruX11ActiveWindowResult;

Display *neru_x11_open_display(void);
void neru_x11_close_display(Display *display);
int neru_x11_query_pointer(Display *display, int *x, int *y);
int neru_x11_move_pointer(Display *display, int x, int y);
// neru_x11_get_active_window reads _NET_ACTIVE_WINDOW from the root window. It
// returns one NeruX11ActiveWindowResult value, typed int like every other entry
// point here so cgo sees the same C.int the rest of this bridge returns. *out is
// written only on NERU_X11_ACTIVE_WINDOW_OK and left untouched otherwise.
int neru_x11_get_active_window(Display *display, Window *out);
unsigned long neru_x11_get_window_pid(Display *display, Window window, int *ok);
// neru_x11_get_window_class returns a heap-allocated copy of the window's
// WM_CLASS "class" field (res_class), or NULL when unavailable. The caller
// owns the returned pointer and must free() it.
char *neru_x11_get_window_class(Display *display, Window window);
NeruX11Monitor *neru_x11_get_monitors(Display *display, int *count);
void neru_x11_free_monitors(NeruX11Monitor *monitors, int count);

// neru_x11_get_focused_window_bounds fills the global (root-relative) bounds of
// the currently focused window (_NET_ACTIVE_WINDOW) via XGetWindowAttributes +
// XTranslateCoordinates. Returns 1 on success, 0 when there is no active window
// or its geometry could not be queried.
int neru_x11_get_focused_window_bounds(Display *display, int *x, int *y, int *w, int *h);

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

// NeruX11ScreenMonitor watches RandR screen-configuration changes (monitors
// added/removed/resized/moved) on a dedicated X11 connection and thread, pushing
// a byte down a self-pipe on each change so callers can wake and re-enumerate
// instead of polling. Opaque to Go; fields live in x11_system.c. This mirrors
// NeruX11FocusMonitor structurally, differing only in the event it selects.
typedef struct NeruX11ScreenMonitor NeruX11ScreenMonitor;

// neru_x11_screen_monitor_start opens a dedicated display, subscribes to RandR
// RRScreenChangeNotify on the root window, and spawns a thread that signals its
// self-pipe on each screen-configuration change. Returns NULL when DISPLAY is
// unset, X11 is unavailable, or the RandR extension is missing. The returned
// monitor is owned by the caller and released with neru_x11_screen_monitor_stop.
NeruX11ScreenMonitor *neru_x11_screen_monitor_start(void);

// neru_x11_screen_monitor_fd returns a readable fd that becomes readable on
// screen-configuration changes, or -1. Owned by the monitor; do not close it.
int neru_x11_screen_monitor_fd(NeruX11ScreenMonitor *monitor);

// neru_x11_screen_monitor_stop signals the thread to exit, joins it, and frees
// the monitor. Safe to call with NULL.
void neru_x11_screen_monitor_stop(NeruX11ScreenMonitor *monitor);

#endif /* X11_SYSTEM_H */
