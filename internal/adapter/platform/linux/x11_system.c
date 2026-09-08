#include "x11_system.h"

#include "x11_error_trap.h"

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
#include <time.h>
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

// neru_x11_discover_pointer learns the global pointer position on an X server
// that cannot answer XQueryPointer truthfully. Xwayland only sees the pointer
// while it is over an X window, so the query returns wherever it last was, or
// the screen centre before it ever was. Mapping a full-root override-redirect
// window that accepts input makes the compositor send the pointer's entry, and
// the position that arrives with it (or with the motion right behind it) is
// the truth. The window is unmapped again before returning; it draws nothing
// and takes no focus, so nothing on screen notices. timeout_ms bounds the wait.
// NERU_X11_POINTER_SETTLE_MS is how long discovery waits after the pointer's
// entry for the motion that carries its real position.
#define NERU_X11_POINTER_SETTLE_MS 40

int neru_x11_discover_pointer(Display *display, int timeout_ms, int *x, int *y) {
	int screen = DefaultScreen(display);
	Window root = neru_x11_root_window(display);

	// InputOutput, not InputOnly: Xwayland gives a Wayland surface only to
	// windows that can be drawn, and a window with no surface is one the
	// compositor never sends the pointer into. A 32-bit visual with no
	// background keeps the surface fully transparent for the frame it lives.
	XVisualInfo visual_info;
	if (!XMatchVisualInfo(display, screen, 32, TrueColor, &visual_info)) {
		return 0;
	}

	// The trap covers creation and map only, so the wait below never holds
	// the process-wide trap lock; every resource here is our own, so the one
	// error that can arrive is an allocation failure, and that has to become
	// a failed sync rather than the exit the default handler performs.
	XSetWindowAttributes attrs;
	attrs.override_redirect = True;
	attrs.event_mask = EnterWindowMask | PointerMotionMask;
	attrs.background_pixel = 0;
	attrs.border_pixel = 0;
	neru_x11_error_trap_begin(display);
	attrs.colormap = XCreateColormap(display, root, visual_info.visual, AllocNone);
	Window probe = XCreateWindow(
	    display, root, 0, 0, (unsigned int)DisplayWidth(display, screen), (unsigned int)DisplayHeight(display, screen),
	    0, visual_info.depth, InputOutput, visual_info.visual,
	    CWOverrideRedirect | CWEventMask | CWColormap | CWBackPixel | CWBorderPixel, &attrs);
	XMapRaised(display, probe);
	if (neru_x11_error_trap_end(display)) {
		neru_x11_error_trap_begin(display);
		XDestroyWindow(display, probe);
		XFreeColormap(display, attrs.colormap);
		neru_x11_error_trap_end(display);
		return 0;
	}

	// The entry event carries Xwayland's last known position, which is where
	// the pointer left the previous X window, or the screen centre before it
	// ever entered one; the motion Xwayland sends right behind it carries the
	// position the compositor just reported. So an entry is provisional, and
	// the wait continues a little for the motion that corrects it.
	int found = 0;
	int settled = 0;
	int fd = ConnectionNumber(display);
	struct timespec start;
	clock_gettime(CLOCK_MONOTONIC, &start);
	long entered_at_ms = -1;
	for (;;) {
		while (XPending(display) > 0) {
			XEvent event;
			XNextEvent(display, &event);
			if (event.type == MotionNotify && event.xmotion.window == probe) {
				*x = event.xmotion.x_root;
				*y = event.xmotion.y_root;
				found = 1;
				settled = 1;
			} else if (event.type == EnterNotify && event.xcrossing.window == probe && !found) {
				*x = event.xcrossing.x_root;
				*y = event.xcrossing.y_root;
				found = 1;
			}
		}
		if (settled) {
			break;
		}
		struct timespec now;
		clock_gettime(CLOCK_MONOTONIC, &now);
		long elapsed_ms = (now.tv_sec - start.tv_sec) * 1000L + (now.tv_nsec - start.tv_nsec) / 1000000L;
		if (found && entered_at_ms < 0) {
			entered_at_ms = elapsed_ms;
		}
		if (elapsed_ms >= timeout_ms || (found && elapsed_ms - entered_at_ms >= NERU_X11_POINTER_SETTLE_MS)) {
			break;
		}
		long remaining_ms = timeout_ms - elapsed_ms;
		if (found && entered_at_ms + NERU_X11_POINTER_SETTLE_MS - elapsed_ms < remaining_ms) {
			remaining_ms = entered_at_ms + NERU_X11_POINTER_SETTLE_MS - elapsed_ms;
		}
		struct pollfd pfd = {fd, POLLIN, 0};
		poll(&pfd, 1, (int)remaining_ms);
	}

	XDestroyWindow(display, probe);
	XFreeColormap(display, attrs.colormap);
	XFlush(display);
	return found;
}

int neru_x11_move_pointer(Display *display, int x, int y) {
	int ok = XTestFakeMotionEvent(display, -1, x, y, CurrentTime);
	XFlush(display);
	return ok;
}

// neru_x11_root_has_live_wm reports whether an EWMH window manager owns this
// display *right now*, by completing the _NET_SUPPORTING_WM_CHECK handshake:
// the root window names a window the window manager created, and that window
// names itself back through the same property.
//
// Something is needed here because _NET_ACTIVE_WINDOW being absent means both
// "no window manager" and "a window manager with nothing focused". Openbox —
// what CI's X11 leg runs — advertises _NET_ACTIVE_WINDOW in _NET_SUPPORTED and
// then writes no such property until something takes focus, while other window
// managers write None instead.
//
// The handshake, and not the mere presence of _NET_SUPPORTED, is what answers
// it. Root-window properties belong to the root window, not to the client that
// wrote them, so a window manager that is killed while any other client keeps
// the session alive leaves every _NET_* property it ever wrote sitting there —
// verified on Xvfb + openbox: SIGKILL the window manager with one connection
// held open and _NET_SUPPORTED is still readable indefinitely afterwards. A
// presence check would call that display "a window manager with nothing
// focused" forever. The window _NET_SUPPORTING_WM_CHECK names is the window
// manager's own and dies with its connection, which is precisely why EWMH
// specifies the self-reference: it distinguishes a live window manager from a
// stale advertisement.
static int neru_x11_root_has_live_wm(Display *display) {
	Atom property = XInternAtom(display, "_NET_SUPPORTING_WM_CHECK", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	Window root = neru_x11_root_window(display);
	int status = XGetWindowProperty(
	    display, root, property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);

	if (status != Success || actual_type != XA_WINDOW || actual_format != 32 || item_count == 0 || data == NULL) {
		if (data != NULL) {
			XFree(data);
		}

		return 0;
	}

	Window wm_window = *((Window *)data);
	XFree(data);

	if (wm_window == None) {
		return 0;
	}

	// A stale property points at a window the server destroyed with the window
	// manager's connection, so this read answers BadWindow. Trapping it is not
	// optional: Xlib's default error handler would exit the daemon.
	data = NULL;

	neru_x11_error_trap_begin(display);
	status = XGetWindowProperty(
	    display, wm_window, property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);
	int trapped = neru_x11_error_trap_end(display);

	if (trapped || status != Success || actual_type != XA_WINDOW || actual_format != 32 || item_count == 0 ||
	    data == NULL) {
		if (data != NULL) {
			XFree(data);
		}

		return 0;
	}

	Window echoed = *((Window *)data);
	XFree(data);

	// The self-reference is the half that survives window-id reuse: a stale
	// root property pointing at an id some other client has since been given
	// answers with that client's property, not with its own id.
	return echoed == wm_window;
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
	// point at — so the _NET_SUPPORTING_WM_CHECK handshake decides which, rather
	// than the caller being told a healthy desktop is broken.
	if (actual_type == None) {
		if (data != NULL) {
			XFree(data);
		}
		return neru_x11_root_has_live_wm(display) ? NERU_X11_ACTIVE_WINDOW_NONE : NERU_X11_ACTIVE_WINDOW_NO_WM;
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

// Every request below is addressed to a window this process does not own, and
// _NET_ACTIVE_WINDOW can name one that is already gone: the window closes
// between the read and this call, or its window manager exited and left the
// property behind pointing at a window that has since died. The X server
// answers BadWindow, and Xlib's default handler calls exit() — reproduced on
// Xvfb, where a stale id took the whole process down. So each of them runs
// inside the shared protocol-error trap and reports the trap instead of dying:
// the pid query names it as its own answer, the ones whose signature carries no
// answer report "nothing to say".

int neru_x11_get_window_pid(Display *display, Window window, unsigned long *out) {
	if (window == 0) {
		// Nobody to ask. The active-window query answers before this one is
		// reached, so this is the same state as a window that died: there is no
		// window there to read a property off.
		return NERU_X11_WINDOW_PID_WINDOW_GONE;
	}

	Atom property = XInternAtom(display, "_NET_WM_PID", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;

	neru_x11_error_trap_begin(display);
	int status = XGetWindowProperty(
	    display, window, property, 0, 1, False, XA_CARDINAL, &actual_type, &actual_format, &item_count, &bytes_after,
	    &data);
	int trapped = neru_x11_error_trap_end(display);

	// One classification, then one free: a trapped call can still have allocated
	// before it errored, so every arm below has something to release.
	int result;

	if (trapped) {
		// A trapped protocol error is BadWindow: the id _NET_ACTIVE_WINDOW named
		// has closed since it was read. That is the failure this call can hit,
		// and it is not the same event as a window that is alive and sets no pid.
		result = NERU_X11_WINDOW_PID_WINDOW_GONE;
	} else if (status != Success) {
		result = NERU_X11_WINDOW_PID_QUERY_FAILED;
	} else if (actual_type == None) {
		// An absent property comes back as Success with an actual type of
		// None — the window is alive and simply does not advertise a pid, which
		// EWMH permits and no client can be made to fix.
		result = NERU_X11_WINDOW_PID_ABSENT;
	} else if (actual_type != XA_CARDINAL || actual_format != 32 || item_count == 0 || data == NULL) {
		// A type or format mismatch is also Success with nothing fetched, so
		// anything that is not the 32-bit CARDINAL EWMH specifies is malformed
		// rather than missing — and reading a narrower format as an unsigned
		// long would read past the property.
		result = NERU_X11_WINDOW_PID_MALFORMED;
	} else {
		*out = *((unsigned long *)data);
		result = NERU_X11_WINDOW_PID_OK;
	}

	if (data != NULL) {
		XFree(data);
	}

	return result;
}

char *neru_x11_get_window_class(Display *display, Window window) {
	if (window == 0) {
		return NULL;
	}

	XClassHint hint = {NULL, NULL};

	neru_x11_error_trap_begin(display);
	int got = XGetClassHint(display, window, &hint);
	int trapped = neru_x11_error_trap_end(display);

	if (got == 0 || trapped) {
		// A trapped call can still have filled the hint before erroring, so
		// release it on the way out rather than assuming it is untouched.
		if (hint.res_name != NULL) {
			XFree(hint.res_name);
		}
		if (hint.res_class != NULL) {
			XFree(hint.res_class);
		}

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

// Titles longer than this many 32-bit units (16 KiB) are truncated. The
// AT-SPI frame name Neru compares against is far shorter than that.
#define NERU_X11_TITLE_MAX_LONGS 4096

char *neru_x11_get_window_title(Display *display, Window window) {
	if (window == 0) {
		return NULL;
	}

	Atom property = XInternAtom(display, "_NET_WM_NAME", False);
	Atom utf8 = XInternAtom(display, "UTF8_STRING", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;

	neru_x11_error_trap_begin(display);
	int status = XGetWindowProperty(
	    display, window, property, 0, NERU_X11_TITLE_MAX_LONGS, False, utf8, &actual_type, &actual_format, &item_count,
	    &bytes_after, &data);
	int trapped = neru_x11_error_trap_end(display);

	if (trapped) {
		// BadWindow: the window closed between the active-window read and this
		// one. A window that is gone has no title to fall back to.
		if (data != NULL) {
			XFree(data);
		}
		return NULL;
	}

	char *title = NULL;
	if (status == Success && actual_type == utf8 && actual_format == 8 && item_count > 0 && data != NULL) {
		// XGetWindowProperty NUL-terminates the returned data; item_count is
		// the byte length for an 8-bit property.
		title = strndup((const char *)data, item_count);
	}
	if (data != NULL) {
		XFree(data);
	}
	if (title != NULL) {
		return title;
	}

	// Older toolkits set only the ICCCM WM_NAME (Latin-1 or COMPOUND_TEXT).
	// XFetchName hands back its raw bytes. AT-SPI reports the same bytes for
	// those windows, so the comparison still lines up.
	char *wm_name = NULL;
	neru_x11_error_trap_begin(display);
	int got = XFetchName(display, window, &wm_name);
	trapped = neru_x11_error_trap_end(display);

	if (got != 0 && !trapped && wm_name != NULL && wm_name[0] != '\0') {
		title = strdup(wm_name);
	}
	if (wm_name != NULL) {
		XFree(wm_name);
	}

	return title;
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

int neru_x11_get_window_bounds(Display *display, Window window, int *x, int *y, int *w, int *h) {
	if (window == 0) {
		return 0;
	}

	// attrs.x/y are relative to the parent; translate the window origin into
	// root coordinates so the bounds are global across a multi-monitor layout.
	// Both requests share one trapped section: the window can die between them
	// just as easily as before the first.
	XWindowAttributes attrs;
	int root_x = 0;
	int root_y = 0;
	Window child;

	neru_x11_error_trap_begin(display);
	int got_attrs = XGetWindowAttributes(display, window, &attrs);
	int translated =
	    got_attrs != 0 ? XTranslateCoordinates(display, window, attrs.root, 0, 0, &root_x, &root_y, &child) : 0;
	int trapped = neru_x11_error_trap_end(display);

	if (got_attrs == 0 || translated == 0 || trapped) {
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
