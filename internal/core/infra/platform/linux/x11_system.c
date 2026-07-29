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

	if (status != Success || data == NULL || item_count == 0) {
		if (data != NULL) {
			XFree(data);
		}
		return 0;
	}

	*out = *((Window *)data);
	XFree(data);

	if (*out == 0) {
		return 0;  // Invalid/No focused window
	}

	return 1;
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

	for (;;) {
		fds[0].revents = 0;
		fds[1].revents = 0;

		int pr = poll(fds, 2, -1);
		if (pr < 0) {
			if (errno == EINTR) {
				continue;
			}
			break;
		}

		if (fds[1].revents & POLLIN) {
			break;  // stop() signaled.
		}

		if (fds[0].revents & (POLLERR | POLLHUP | POLLNVAL)) {
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
