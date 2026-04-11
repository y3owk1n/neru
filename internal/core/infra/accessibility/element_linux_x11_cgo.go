//go:build linux && cgo

package accessibility

/*
#cgo linux LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <stdlib.h>
#include <string.h>

static Display* neru_ax_open_display(void) {
	return XOpenDisplay(NULL);
}

static void neru_ax_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static int neru_ax_get_active_window(Display *display, Window *out) {
	Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
	Atom actual_type;
	int actual_format;
	unsigned long item_count;
	unsigned long bytes_after;
	unsigned char *data = NULL;
	Window root = RootWindow(display, DefaultScreen(display));
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
	return 1;
}

static unsigned long neru_ax_window_pid(Display *display, Window window, int *ok) {
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

static char* neru_ax_window_class(Display *display, Window window) {
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
*/
import "C"

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"
)

func linuxFocusedApplicationIdentity() (string, int) {
	if os.Getenv("DISPLAY") == "" {
		return "", 0
	}

	display := C.neru_ax_open_display()
	if display == nil {
		return "", 0
	}
	defer C.neru_ax_close_display(display) //nolint:nlreturn

	var window C.Window
	if C.neru_ax_get_active_window(display, &window) == 0 {
		return "", 0
	}

	className := C.neru_ax_window_class(display, window)
	bundleID := ""
	if className != nil {
		bundleID = C.GoString(className)
		C.free(unsafe.Pointer(className))
	}

	var ok C.int
	pid := int(C.neru_ax_window_pid(display, window, &ok))

	return bundleID, pid
}

func linuxApplicationBundleIdentifier(pid int) string {
	if pid <= 0 {
		return ""
	}

	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}

	return filepath.Base(parts[0])
}
