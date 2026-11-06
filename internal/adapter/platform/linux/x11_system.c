#include "x11_system.h"

#include "x11_error_trap.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xresource.h>
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

double neru_x11_display_scale(Display *display) {
	if (display == NULL) {
		return 1.0;
	}

	double scale = 1.0;
	char *resource_string = XResourceManagerString(display);
	if (resource_string != NULL) {
		XrmInitialize();
		XrmDatabase db = XrmGetStringDatabase(resource_string);
		if (db != NULL) {
			char *type = NULL;
			XrmValue value;
			if (XrmGetResource(db, "Xft.dpi", "Xft.Dpi", &type, &value) && value.addr != NULL) {
				double dpi = atof(value.addr);
				if (dpi > 0.0) {
					scale = dpi / 96.0;
				}
			}
			XrmDestroyDatabase(db);
		}
	}

	if (scale < 1.0) {
		scale = 1.0;
	}
	if (scale > 4.0) {
		scale = 4.0;
	}
	return scale;
}

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
// over an X window, so the query returns where it last was, or the screen
// centre. Mapping a full-root override-redirect input window makes the
// compositor send the pointer's entry, which carries the truth. The window
// draws nothing, takes no focus, and is unmapped before returning.
// NERU_X11_POINTER_SETTLE_MS is how long discovery waits after the pointer's
// entry for the motion that carries its real position.
#define NERU_X11_POINTER_SETTLE_MS 40

int neru_x11_discover_pointer(Display *display, int timeout_ms, int *x, int *y) {
	int screen = DefaultScreen(display);
	Window root = neru_x11_root_window(display);

	// InputOutput, not InputOnly, because Xwayland gives a Wayland surface only
	// to windows that can be drawn, and a window with no surface is one the
	// compositor never sends the pointer into. A 32-bit visual with no
	// background keeps the surface fully transparent for the frame it lives.
	XVisualInfo visual_info;
	if (!XMatchVisualInfo(display, screen, 32, TrueColor, &visual_info)) {
		return 0;
	}

	// The trap covers creation and map only, so the wait never holds the
	// process-wide trap lock. The only possible error is an allocation failure,
	// which must fail the sync rather than exit.
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

	// The entry carries Xwayland's last known position, which may be stale. The
	// motion right behind it carries the compositor's fresh one, so the wait
	// continues briefly for it.
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
// display *right now*, by completing the _NET_SUPPORTING_WM_CHECK handshake.
// The root names a window the window manager created, and that window names
// itself back through the same property.
//
// An absent _NET_ACTIVE_WINDOW means both "no window manager" and "nothing
// focused". Openbox, which CI's X11 leg runs, writes no such property until
// something takes focus.
//
// The presence of _NET_SUPPORTED cannot answer it. Root properties outlive the
// client that wrote them, so a killed window manager leaves them behind
// (verified on Xvfb + openbox with one other connection held open). The check
// window dies with the window manager's connection, which is why EWMH
// specifies the self-reference.
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
	// optional, because Xlib's default error handler would exit the daemon.
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

	// The self-reference is the half that survives window-id reuse. A stale
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

	// An absent property is Success with type None, both with no window manager
	// and with nothing focused, so the handshake decides which.
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

// Every request below targets a window this process does not own, and
// _NET_ACTIVE_WINDOW can name one already gone (closed since the read, or left
// behind by an exited window manager). The server answers BadWindow and Xlib's
// default handler calls exit(), as reproduced on Xvfb. So each runs inside the
// shared trap. The pid query reports it as its own answer, and the others
// report "nothing to say".

int neru_x11_get_window_pid(Display *display, Window window, unsigned long *out) {
	if (window == 0) {
		// Nobody to ask, the same state as a window that died.
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

	// One classification, then one free, because a trapped call can still have
	// allocated before it errored, so every arm below has something to release.
	int result;

	if (trapped) {
		// A trapped error is BadWindow. The window closed since it was read,
		// which differs from a live window that sets no pid.
		result = NERU_X11_WINDOW_PID_WINDOW_GONE;
	} else if (status != Success) {
		result = NERU_X11_WINDOW_PID_QUERY_FAILED;
	} else if (actual_type == None) {
		// Absent is Success with type None. The window is alive and does not
		// advertise a pid, which EWMH permits.
		result = NERU_X11_WINDOW_PID_ABSENT;
	} else if (actual_type != XA_CARDINAL || actual_format != 32 || item_count == 0 || data == NULL) {
		// A type or format mismatch is also Success with nothing fetched, so
		// anything that is not the 32-bit CARDINAL EWMH specifies is malformed
		// rather than missing, and reading a narrower format as an unsigned
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
		// XGetWindowProperty NUL-terminates the returned data, and item_count is
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

	// attrs.x/y are parent-relative, so translate the origin into root
	// coordinates. Both requests share one trapped section, since the window
	// can die between them.
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

// The monitor thread polls the X11 connection and the quit pipe. Only this
// thread uses the dedicated display, so no XInitThreads. Each
// _NET_ACTIVE_WINDOW PropertyNotify writes a byte to the event pipe, coalescing
// bursts, and the Go reader re-queries the active window on wake.
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
			break;  // stop() signaled a clean exit, and stop() closes the pipes.
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

	// If the X connection died rather than stopped, close the event pipe's
	// write end so the Go reader sees POLLHUP and restores its polling fallback.
	// -1 keeps a later stop() from closing a possibly-reused fd. Only this
	// thread writes the pipe, so no lock is needed.
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

// Same shape as the focus monitor. On each RRScreenChangeNotify it refreshes
// Xlib's cached configuration and signals the event pipe, and the Go reader
// re-enumerates monitors on wake.
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
			break;  // stop() signaled a clean exit, and stop() closes the pipes.
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

	// As in the focus monitor, a dead connection closes the write end so the
	// Go reader sees POLLHUP.
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
		return NULL;  // Without RandR there are no screen-change events to deliver.
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
