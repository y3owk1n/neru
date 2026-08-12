#include "x11_system.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/extensions/Xrandr.h>
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

Display *neru_x11_open_display(void) { return XOpenDisplay(NULL); }

void neru_x11_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static Window neru_x11_root_window(Display *display) { return RootWindow(display, DefaultScreen(display)); }

int neru_x11_query_pointer(Display *display, int *x, int *y) {
	Window root = neru_x11_root_window(display);
	Window root_return;
	Window child_return;
	int win_x, win_y;
	unsigned int mask_return;

	return XQueryPointer(display, root, &root_return, &child_return, x, y, &win_x, &win_y, &mask_return);
}

int neru_x11_move_pointer(Display *display, int x, int y) {
	int ok = XTestFakeMotionEvent(display, -1, x, y, CurrentTime);
	XFlush(display);
	return ok;
}

// neru_x11_root_has_ewmh reports whether a window manager has claimed EWMH on
// this display, by looking for _NET_SUPPORTED on the root window.
//
// This is the only reliable way to tell "no window manager" from "a window
// manager with nothing focused", because _NET_ACTIVE_WINDOW being absent means
// both. Openbox — what CI's X11 leg runs — advertises _NET_ACTIVE_WINDOW in
// _NET_SUPPORTED but writes no such property until something takes focus, while
// other window managers write None instead. _NET_SUPPORTED is the property the
// spec makes a window manager set, and it is set for the WM's whole lifetime.
static int neru_x11_root_has_ewmh(Display *display) {
	Atom property = XInternAtom(display, "_NET_SUPPORTED", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	Window root = neru_x11_root_window(display);
	int status = XGetWindowProperty(
	    display, root, property, 0, 1, False, XA_ATOM, &actual_type, &actual_format, &item_count, &bytes_after, &data);

	int present = (status == Success && actual_type == XA_ATOM && item_count > 0);

	if (data != NULL) {
		XFree(data);
	}

	return present;
}

int neru_x11_get_active_window(Display *display, Window *out) {
	Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	Window root = neru_x11_root_window(display);
	int status = XGetWindowProperty(
	    display, root, property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);

	if (status != Success) {
		if (data != NULL) {
			XFree(data);
		}
		return NERU_X11_ACTIVE_WINDOW_QUERY_FAILED;
	}

	// XGetWindowProperty reports an absent property as Success with an actual
	// type of None. Two very different sessions look like that — one with no
	// window manager at all, and one whose window manager simply has nothing to
	// point at — so _NET_SUPPORTED decides which, rather than the caller being
	// told a healthy desktop is broken.
	if (actual_type == None) {
		if (data != NULL) {
			XFree(data);
		}
		return neru_x11_root_has_ewmh(display) ? NERU_X11_ACTIVE_WINDOW_NONE : NERU_X11_ACTIVE_WINDOW_NO_WM;
	}

	// A type or format mismatch also comes back as Success with nothing
	// fetched, so anything that is not the single 32-bit WINDOW value EWMH
	// specifies is a malformed property rather than a missing one.
	if (actual_type != XA_WINDOW || actual_format != 32 || item_count == 0 || data == NULL) {
		if (data != NULL) {
			XFree(data);
		}
		return NERU_X11_ACTIVE_WINDOW_MALFORMED;
	}

	Window active = *((Window *)data);
	XFree(data);

	if (active == None) {
		return NERU_X11_ACTIVE_WINDOW_NONE;  // A live desktop with nothing focused.
	}

	*out = active;

	return NERU_X11_ACTIVE_WINDOW_OK;
}

unsigned long neru_x11_get_window_pid(Display *display, Window window, int *ok) {
	if (window == 0) {
		*ok = 0;
		return 0;
	}

	Atom property = XInternAtom(display, "_NET_WM_PID", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	int status = XGetWindowProperty(
	    display, window, property, 0, 1, False, XA_CARDINAL, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);

	if (status != Success || data == NULL || item_count == 0) {
		if (data != NULL) {
			XFree(data);
		}
		*ok = 0;
		return 0;
	}

	*ok = 1;
	unsigned long pid = *((unsigned long *)data);
	XFree(data);

	return pid;
}

char *neru_x11_get_window_class(Display *display, Window window) {
	if (window == 0) {
		return NULL;
	}

	XClassHint hint;
	if (XGetClassHint(display, window, &hint) == 0) {
		return NULL;
	}

	char *class_name = NULL;
	if (hint.res_class != NULL) {
		class_name = strdup(hint.res_class);
	}

	if (hint.res_name != NULL) {
		XFree(hint.res_name);
	}
	if (hint.res_class != NULL) {
		XFree(hint.res_class);
	}

	return class_name;
}

NeruX11Monitor *neru_x11_get_monitors(Display *display, int *count) {
	Window root = neru_x11_root_window(display);
	int monitor_count = 0;
	XRRMonitorInfo *monitors = XRRGetMonitors(display, root, True, &monitor_count);
	if (monitors == NULL || monitor_count <= 0) {
		*count = 0;
		return NULL;
	}

	NeruX11Monitor *result = calloc((size_t)monitor_count, sizeof(NeruX11Monitor));
	if (result == NULL) {
		XRRFreeMonitors(monitors);
		*count = 0;
		return NULL;
	}

	for (int i = 0; i < monitor_count; i++) {
		result[i].x = monitors[i].x;
		result[i].y = monitors[i].y;
		result[i].width = monitors[i].width;
		result[i].height = monitors[i].height;
		result[i].primary = monitors[i].primary;
		if (monitors[i].name != None) {
			char *atom_name = XGetAtomName(display, monitors[i].name);
			if (atom_name != NULL) {
				result[i].name = strdup(atom_name);
				XFree(atom_name);
			}
		}
	}

	XRRFreeMonitors(monitors);
	*count = monitor_count;

	return result;
}

int neru_x11_get_focused_window_bounds(Display *display, int *x, int *y, int *w, int *h) {
	// Bounds callers get one bit either way: without a focused window there is
	// no geometry to report, and they widen to the active screen. Which of the
	// four answers came back is the focused-app path's business, not theirs.
	Window window;
	if (neru_x11_get_active_window(display, &window) != NERU_X11_ACTIVE_WINDOW_OK) {
		return 0;
	}

	XWindowAttributes attrs;
	if (XGetWindowAttributes(display, window, &attrs) == 0) {
		return 0;
	}

	// attrs.x/y are relative to the parent; translate the window origin into
	// root coordinates so the bounds are global across a multi-monitor layout.
	int root_x = 0;
	int root_y = 0;
	Window child;
	if (XTranslateCoordinates(display, window, attrs.root, 0, 0, &root_x, &root_y, &child) == 0) {
		return 0;
	}

	*x = root_x;
	*y = root_y;
	*w = attrs.width;
	*h = attrs.height;

	return 1;
}

void neru_x11_free_monitors(NeruX11Monitor *monitors, int count) {
	if (monitors == NULL) {
		return;
	}

	for (int i = 0; i < count; i++) {
		free(monitors[i].name);
	}

	free(monitors);
}

// ---------- Focused-window monitor ----------

struct NeruX11FocusMonitor {
	Display *display;
	Window root;
	Atom active_atom;
	int event_pipe[2];  // [0] read (exposed to Go), [1] write (thread)
	int quit_pipe[2];   // [0] read (thread), [1] write (stop)
	pthread_t thread;
	int thread_started;
};

static void neru_x11_set_nonblock_cloexec(int fd) {
	if (fd < 0) {
		return;
	}

	int flags = fcntl(fd, F_GETFL, 0);
	if (flags != -1) {
		fcntl(fd, F_SETFL, flags | O_NONBLOCK);
	}

	int fdflags = fcntl(fd, F_GETFD, 0);
	if (fdflags != -1) {
		fcntl(fd, F_SETFD, fdflags | FD_CLOEXEC);
	}
}

static void neru_x11_close_pipe(int pipe_fds[2]) {
	if (pipe_fds[0] >= 0) {
		close(pipe_fds[0]);
		pipe_fds[0] = -1;
	}
	if (pipe_fds[1] >= 0) {
		close(pipe_fds[1]);
		pipe_fds[1] = -1;
	}
}

// The monitor thread blocks in poll() on the X11 connection and the quit pipe.
// The dedicated display is used only by this thread, so no XInitThreads is
// needed. On each _NET_ACTIVE_WINDOW PropertyNotify it writes a byte to the
// event pipe (coalescing a burst into one signal); the Go reader re-queries the
// live active window on wake.
static void *neru_x11_focus_loop(void *arg) {
	NeruX11FocusMonitor *m = (NeruX11FocusMonitor *)arg;
	int xfd = ConnectionNumber(m->display);

	struct pollfd fds[2];
	fds[0].fd = xfd;
	fds[0].events = POLLIN;
	fds[1].fd = m->quit_pipe[0];
	fds[1].events = POLLIN;

	int connection_lost = 0;

	for (;;) {
		fds[0].revents = 0;
		fds[1].revents = 0;

		int pr = poll(fds, 2, -1);
		if (pr < 0) {
			if (errno == EINTR) {
				continue;
			}
			connection_lost = 1;
			break;
		}

		if (fds[1].revents & POLLIN) {
			break;  // stop() signaled — clean exit; stop() closes the pipes.
		}

		if (fds[0].revents & (POLLERR | POLLHUP | POLLNVAL)) {
			connection_lost = 1;
			break;  // X connection died.
		}

		if (!(fds[0].revents & POLLIN)) {
			continue;
		}

		int changed = 0;
		while (XPending(m->display) > 0) {
			XEvent ev;
			XNextEvent(m->display, &ev);
			if (ev.type == PropertyNotify && ev.xproperty.window == m->root && ev.xproperty.atom == m->active_atom) {
				changed = 1;
			}
		}

		if (changed) {
			char b = 1;
			ssize_t n = write(m->event_pipe[1], &b, 1);
			(void)n;  // Best-effort: EAGAIN means a prior byte is still unread.
		}
	}

	// If the X connection died (as opposed to a clean stop), close the event
	// pipe's write end so the Go reader observes POLLHUP and restores its
	// polling fallback instead of silently degrading to the safety-sample
	// interval. Cleared to -1 so a later stop()'s close is a no-op (no
	// double-close of a possibly-reused fd). Safe without locking: only this
	// thread writes the pipe, and it has stopped writing by here.
	if (connection_lost && m->event_pipe[1] >= 0) {
		close(m->event_pipe[1]);
		m->event_pipe[1] = -1;
	}

	return NULL;
}

NeruX11FocusMonitor *neru_x11_focus_monitor_start(void) {
	if (getenv("DISPLAY") == NULL) {
		return NULL;
	}

	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		return NULL;
	}

	NeruX11FocusMonitor *m = calloc(1, sizeof(NeruX11FocusMonitor));
	if (m == NULL) {
		XCloseDisplay(display);
		return NULL;
	}

	m->display = display;
	m->root = neru_x11_root_window(display);
	m->active_atom = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
	m->event_pipe[0] = -1;
	m->event_pipe[1] = -1;
	m->quit_pipe[0] = -1;
	m->quit_pipe[1] = -1;

	if (pipe(m->event_pipe) != 0 || pipe(m->quit_pipe) != 0) {
		neru_x11_close_pipe(m->event_pipe);
		neru_x11_close_pipe(m->quit_pipe);
		XCloseDisplay(display);
		free(m);
		return NULL;
	}

	neru_x11_set_nonblock_cloexec(m->event_pipe[0]);
	neru_x11_set_nonblock_cloexec(m->event_pipe[1]);
	neru_x11_set_nonblock_cloexec(m->quit_pipe[0]);
	neru_x11_set_nonblock_cloexec(m->quit_pipe[1]);

	XSelectInput(display, m->root, PropertyChangeMask);
	XFlush(display);

	if (pthread_create(&m->thread, NULL, neru_x11_focus_loop, m) != 0) {
		neru_x11_close_pipe(m->event_pipe);
		neru_x11_close_pipe(m->quit_pipe);
		XCloseDisplay(display);
		free(m);
		return NULL;
	}
	m->thread_started = 1;

	return m;
}

int neru_x11_focus_monitor_fd(NeruX11FocusMonitor *monitor) {
	if (monitor == NULL) {
		return -1;
	}
	return monitor->event_pipe[0];
}

void neru_x11_focus_monitor_stop(NeruX11FocusMonitor *monitor) {
	if (monitor == NULL) {
		return;
	}

	if (monitor->thread_started) {
		char b = 1;
		ssize_t n = write(monitor->quit_pipe[1], &b, 1);
		(void)n;
		pthread_join(monitor->thread, NULL);
		monitor->thread_started = 0;
	}

	neru_x11_close_pipe(monitor->event_pipe);
	neru_x11_close_pipe(monitor->quit_pipe);

	if (monitor->display != NULL) {
		XCloseDisplay(monitor->display);
		monitor->display = NULL;
	}

	free(monitor);
}

// ---------- Screen-configuration monitor (RandR) ----------

struct NeruX11ScreenMonitor {
	Display *display;
	Window root;
	int randr_event_base;
	int event_pipe[2];  // [0] read (exposed to Go), [1] write (thread)
	int quit_pipe[2];   // [0] read (thread), [1] write (stop)
	pthread_t thread;
	int thread_started;
};

// The monitor thread blocks in poll() on the X11 connection and the quit pipe.
// On each RRScreenChangeNotify it keeps Xlib's cached configuration current
// (XRRUpdateConfiguration) and writes a byte to the event pipe (coalescing a
// burst into one signal); the Go reader re-enumerates monitors on wake.
static void *neru_x11_screen_loop(void *arg) {
	NeruX11ScreenMonitor *m = (NeruX11ScreenMonitor *)arg;
	int xfd = ConnectionNumber(m->display);

	struct pollfd fds[2];
	fds[0].fd = xfd;
	fds[0].events = POLLIN;
	fds[1].fd = m->quit_pipe[0];
	fds[1].events = POLLIN;

	int connection_lost = 0;

	for (;;) {
		fds[0].revents = 0;
		fds[1].revents = 0;

		int pr = poll(fds, 2, -1);
		if (pr < 0) {
			if (errno == EINTR) {
				continue;
			}
			connection_lost = 1;
			break;
		}

		if (fds[1].revents & POLLIN) {
			break;  // stop() signaled — clean exit; stop() closes the pipes.
		}

		if (fds[0].revents & (POLLERR | POLLHUP | POLLNVAL)) {
			connection_lost = 1;
			break;  // X connection died.
		}

		if (!(fds[0].revents & POLLIN)) {
			continue;
		}

		int changed = 0;
		while (XPending(m->display) > 0) {
			XEvent ev;
			XNextEvent(m->display, &ev);
			// Let Xlib refresh its cached screen configuration so a subsequent
			// XRRGetMonitors on any connection reflects the new layout.
			XRRUpdateConfiguration(&ev);
			if (ev.type == m->randr_event_base + RRScreenChangeNotify) {
				changed = 1;
			}
		}

		if (changed) {
			char b = 1;
			ssize_t n = write(m->event_pipe[1], &b, 1);
			(void)n;  // Best-effort: EAGAIN means a prior byte is still unread.
		}
	}

	// If the X connection died (as opposed to a clean stop), close the event
	// pipe's write end so the Go reader observes POLLHUP. Cleared to -1 so a
	// later stop()'s close is a no-op. Safe without locking: only this thread
	// writes the pipe, and it has stopped writing by here.
	if (connection_lost && m->event_pipe[1] >= 0) {
		close(m->event_pipe[1]);
		m->event_pipe[1] = -1;
	}

	return NULL;
}

NeruX11ScreenMonitor *neru_x11_screen_monitor_start(void) {
	if (getenv("DISPLAY") == NULL) {
		return NULL;
	}

	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		return NULL;
	}

	int event_base = 0;
	int error_base = 0;
	if (XRRQueryExtension(display, &event_base, &error_base) == 0) {
		XCloseDisplay(display);
		return NULL;  // RandR unavailable — no screen-change events to deliver.
	}

	NeruX11ScreenMonitor *m = calloc(1, sizeof(NeruX11ScreenMonitor));
	if (m == NULL) {
		XCloseDisplay(display);
		return NULL;
	}

	m->display = display;
	m->root = neru_x11_root_window(display);
	m->randr_event_base = event_base;
	m->event_pipe[0] = -1;
	m->event_pipe[1] = -1;
	m->quit_pipe[0] = -1;
	m->quit_pipe[1] = -1;

	if (pipe(m->event_pipe) != 0 || pipe(m->quit_pipe) != 0) {
		neru_x11_close_pipe(m->event_pipe);
		neru_x11_close_pipe(m->quit_pipe);
		XCloseDisplay(display);
		free(m);
		return NULL;
	}

	neru_x11_set_nonblock_cloexec(m->event_pipe[0]);
	neru_x11_set_nonblock_cloexec(m->event_pipe[1]);
	neru_x11_set_nonblock_cloexec(m->quit_pipe[0]);
	neru_x11_set_nonblock_cloexec(m->quit_pipe[1]);

	XRRSelectInput(display, m->root, RRScreenChangeNotifyMask);
	XFlush(display);

	if (pthread_create(&m->thread, NULL, neru_x11_screen_loop, m) != 0) {
		neru_x11_close_pipe(m->event_pipe);
		neru_x11_close_pipe(m->quit_pipe);
		XCloseDisplay(display);
		free(m);
		return NULL;
	}
	m->thread_started = 1;

	return m;
}

int neru_x11_screen_monitor_fd(NeruX11ScreenMonitor *monitor) {
	if (monitor == NULL) {
		return -1;
	}
	return monitor->event_pipe[0];
}

void neru_x11_screen_monitor_stop(NeruX11ScreenMonitor *monitor) {
	if (monitor == NULL) {
		return;
	}

	if (monitor->thread_started) {
		char b = 1;
		ssize_t n = write(monitor->quit_pipe[1], &b, 1);
		(void)n;
		pthread_join(monitor->thread, NULL);
		monitor->thread_started = 0;
	}

	neru_x11_close_pipe(monitor->event_pipe);
	neru_x11_close_pipe(monitor->quit_pipe);

	if (monitor->display != NULL) {
		XCloseDisplay(monitor->display);
		monitor->display = NULL;
	}

	free(monitor);
}
