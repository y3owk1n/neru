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
// _NET_ACTIVE_WINDOW can give. Nothing focused is an answer, not a failure, or
// a user who clicked their wallpaper looks like a broken X server. The failures
// stay apart too, because the fixes differ: start an EWMH window manager, check
// the display server, or fix whatever wrote a non-conforming property.
typedef enum {
	NERU_X11_ACTIVE_WINDOW_OK = 0,            // *out holds a live window id
	NERU_X11_ACTIVE_WINDOW_NONE = 1,          // queried fine, nothing is focused
	NERU_X11_ACTIVE_WINDOW_NO_WM = 2,         // no live EWMH window manager owns this display
	NERU_X11_ACTIVE_WINDOW_QUERY_FAILED = 3,  // XGetWindowProperty failed
	NERU_X11_ACTIVE_WINDOW_MALFORMED = 4,     // present, but not a 32-bit WINDOW value
} NeruX11ActiveWindowResult;

// NeruX11WindowPIDResult does the same for _NET_WM_PID. A window that sets no
// pid answered the question, since the property is a convention older clients
// omit and a remote client cannot set usefully. A window that closed under us
// is a failure.
typedef enum {
	NERU_X11_WINDOW_PID_OK = 0,            // *out holds the window's pid
	NERU_X11_WINDOW_PID_ABSENT = 1,        // read fine, the window sets no _NET_WM_PID
	NERU_X11_WINDOW_PID_WINDOW_GONE = 2,   // the request errored: the window has closed
	NERU_X11_WINDOW_PID_QUERY_FAILED = 3,  // XGetWindowProperty failed
	NERU_X11_WINDOW_PID_MALFORMED = 4,     // present, but not a 32-bit CARDINAL value
} NeruX11WindowPIDResult;

Display *neru_x11_open_display(void);
// neru_x11_display_scale returns the desktop-wide HiDPI UI scale, Xft.dpi / 96
// clamped to [1.0, 4.0], and 1.0 when Xft.dpi is unset. X11 has a single
// coordinate space in physical pixels and no authoritative per-monitor scale,
// so this one factor is what an apparent size is multiplied by everywhere.
double neru_x11_display_scale(Display *display);
void neru_x11_close_display(Display *display);
int neru_x11_query_pointer(Display *display, int *x, int *y);
int neru_x11_discover_pointer(Display *display, int timeout_ms, int *x, int *y);
int neru_x11_move_pointer(Display *display, int x, int y);
// neru_x11_get_active_window reads _NET_ACTIVE_WINDOW from the root window. It
// returns a NeruX11ActiveWindowResult typed int, like every entry point here,
// and writes *out only on NERU_X11_ACTIVE_WINDOW_OK.
int neru_x11_get_active_window(Display *display, Window *out);
// neru_x11_get_window_pid reads _NET_WM_PID off window. It returns one
// NeruX11WindowPIDResult value, typed int like every other entry point here, and
// writes *out only on NERU_X11_WINDOW_PID_OK.
int neru_x11_get_window_pid(Display *display, Window window, unsigned long *out);
// neru_x11_get_window_class returns a heap-allocated copy of the window's
// WM_CLASS "class" field (res_class), or NULL when unavailable. The caller
// owns the returned pointer and must free() it.
char *neru_x11_get_window_class(Display *display, Window window);
// neru_x11_get_window_title returns a heap-allocated copy of the window's
// _NET_WM_NAME (UTF-8), falling back to the ICCCM WM_NAME when the EWMH
// property is absent, or NULL when the window sets neither or has closed. The
// caller owns the returned pointer and must free() it.
char *neru_x11_get_window_title(Display *display, Window window);
NeruX11Monitor *neru_x11_get_monitors(Display *display, int *count);
void neru_x11_free_monitors(NeruX11Monitor *monitors, int count);

// neru_x11_get_window_bounds fills the global (root-relative) bounds of window.
// Returns 1 on success, 0 when the window closed between the two requests or
// the server refused them. Callers find the window via
// neru_x11_get_active_window, which alone tells nothing-focused from failure.
int neru_x11_get_window_bounds(Display *display, Window window, int *x, int *y, int *w, int *h);

// NeruX11FocusMonitor watches _NET_ACTIVE_WINDOW from a dedicated X11
// connection and thread, signalling a self-pipe on each change so callers wake
// instead of polling. Opaque to Go.
typedef struct NeruX11FocusMonitor NeruX11FocusMonitor;

// neru_x11_focus_monitor_start opens a dedicated display, subscribes to
// _NET_ACTIVE_WINDOW property changes on the root window, and spawns a thread
// that signals its self-pipe on each change. Returns NULL when DISPLAY is unset
// or X11 is otherwise unavailable. The returned monitor is owned by the caller
// and released with neru_x11_focus_monitor_stop.
NeruX11FocusMonitor *neru_x11_focus_monitor_start(void);

// neru_x11_focus_monitor_fd returns a readable fd that becomes readable on
// active-window changes, or -1. Owned by the monitor, and callers must not close it.
int neru_x11_focus_monitor_fd(NeruX11FocusMonitor *monitor);

// neru_x11_focus_monitor_stop signals the thread to exit, joins it, and frees
// the monitor. Safe to call with NULL.
void neru_x11_focus_monitor_stop(NeruX11FocusMonitor *monitor);

// NeruX11ScreenMonitor mirrors NeruX11FocusMonitor for RandR
// screen-configuration changes (monitors added, removed, resized or moved).
typedef struct NeruX11ScreenMonitor NeruX11ScreenMonitor;

// neru_x11_screen_monitor_start opens a dedicated display, subscribes to RandR
// RRScreenChangeNotify on the root window, and spawns a thread that signals its
// self-pipe on each screen-configuration change. Returns NULL when DISPLAY is
// unset, X11 is unavailable, or the RandR extension is missing. The returned
// monitor is owned by the caller and released with neru_x11_screen_monitor_stop.
NeruX11ScreenMonitor *neru_x11_screen_monitor_start(void);

// neru_x11_screen_monitor_fd returns a readable fd that becomes readable on
// screen-configuration changes, or -1. Owned by the monitor, so do not close it.
int neru_x11_screen_monitor_fd(NeruX11ScreenMonitor *monitor);

// neru_x11_screen_monitor_stop signals the thread to exit, joins it, and frees
// the monitor. Safe to call with NULL.
void neru_x11_screen_monitor_stop(NeruX11ScreenMonitor *monitor);

#endif /* X11_SYSTEM_H */
