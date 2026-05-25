#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/extensions/Xrandr.h>
#include <stdlib.h>
#include <string.h>
#include "x11_system.h"


typedef struct {
	int x;
	int y;
	int width;
	int height;
	int primary;
	char *name;
} NeruX11Monitor;

Display* neru_x11_open_display(void) {
	return XOpenDisplay(NULL);
}

void neru_x11_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static Window neru_x11_root_window(Display *display) {
	return RootWindow(display, DefaultScreen(display));
}

int neru_x11_query_pointer(Display *display, int *x, int *y) {
	Window root = neru_x11_root_window(display);
	Window root_return;
	Window child_return;
	int win_x, win_y;
	unsigned int mask_return;

	return XQueryPointer(
		display,
		root,
		&root_return,
		&child_return,
		x,
		y,
		&win_x,
		&win_y,
		&mask_return
	);
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
		display,
		root,
		property,
		0,
		1,
		False,
		XA_WINDOW,
		&actual_type,
		&actual_format,
		&item_count,
		&bytes_after,
		&data
	);

	if (status != Success || data == NULL || item_count == 0) {
		if (data != NULL) {
			XFree(data);
		}
		return 0;
	}

	*out = *((Window *)data);
	XFree(data);

	if (*out == 0) {
		return 0; // Invalid/No focused window
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
		display,
		window,
		property,
		0,
		1,
		False,
		XA_CARDINAL,
		&actual_type,
		&actual_format,
		&item_count,
		&bytes_after,
		&data
	);

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

static char* neru_x11_get_window_class(Display *display, Window window) {
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

NeruX11Monitor* neru_x11_get_monitors(Display *display, int *count) {
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
