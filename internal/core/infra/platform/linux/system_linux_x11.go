//go:build linux && cgo

package linux

/*
#cgo linux LDFLAGS: -lX11 -lXtst -lXrandr
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>
#include <X11/extensions/Xrandr.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	int x;
	int y;
	int width;
	int height;
	int primary;
	char *name;
} NeruX11Monitor;

static Display* neru_x11_open_display(void) {
	return XOpenDisplay(NULL);
}

static void neru_x11_close_display(Display *display) {
	if (display != NULL) {
		XCloseDisplay(display);
	}
}

static Window neru_x11_root_window(Display *display) {
	return RootWindow(display, DefaultScreen(display));
}

static int neru_x11_query_pointer(Display *display, int *x, int *y) {
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

static int neru_x11_move_pointer(Display *display, int x, int y) {
	int ok = XTestFakeMotionEvent(display, -1, x, y, CurrentTime);
	XFlush(display);
	return ok;
}

static int neru_x11_get_active_window(Display *display, Window *out) {
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

static unsigned long neru_x11_get_window_pid(Display *display, Window window, int *ok) {
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

static NeruX11Monitor* neru_x11_get_monitors(Display *display, int *count) {
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

static void neru_x11_free_monitors(NeruX11Monitor *monitors, int count) {
	if (monitors == NULL) {
		return;
	}

	for (int i = 0; i < count; i++) {
		free(monitors[i].name);
	}

	free(monitors);
}
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

type x11Monitor struct {
	Name    string
	Bounds  image.Rectangle
	Primary bool
}

func x11OpenDisplay() (*C.Display, error) {
	if os.Getenv("DISPLAY") == "" {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"DISPLAY is not set; X11 backend is unavailable",
		)
	}

	display := C.neru_x11_open_display()
	if display == nil {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"failed to open X11 display",
		)
	}

	return display, nil
}

func x11CursorPosition() (image.Point, error) {
	display, err := x11OpenDisplay()
	if err != nil {
		return image.Point{}, err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var x, y C.int
	if C.neru_x11_query_pointer(display, &x, &y) == 0 {
		return image.Point{}, derrors.New(
			derrors.CodeActionFailed,
			"failed to query X11 pointer position",
		)
	}

	return image.Point{X: int(x), Y: int(y)}, nil
}

func x11MoveCursorToPoint(point image.Point) error {
	display, err := x11OpenDisplay()
	if err != nil {
		return err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	if C.neru_x11_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move X11 pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

func x11FocusedApplicationPID() (int, error) {
	display, err := x11OpenDisplay()
	if err != nil {
		return 0, err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var window C.Window
	if C.neru_x11_get_active_window(display, &window) == 0 {
		return 0, derrors.New(
			derrors.CodeActionFailed,
			"failed to query _NET_ACTIVE_WINDOW on X11",
		)
	}

	var ok C.int
	pid := C.neru_x11_get_window_pid(display, window, &ok)
	if ok == 0 {
		return 0, derrors.New(
			derrors.CodeActionFailed,
			"failed to query _NET_WM_PID for active X11 window",
		)
	}

	return int(pid), nil
}

func linuxApplicationNameByPID(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "comm"))
	if err != nil {
		return "", derrors.Wrapf(
			err,
			derrors.CodeActionFailed,
			"failed to read /proc/%d/comm",
			pid,
		)
	}

	return strings.TrimSpace(string(data)), nil
}

func linuxApplicationBundleIDByPID(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "cmdline"))
	if err != nil {
		return "", derrors.Wrapf(
			err,
			derrors.CodeActionFailed,
			"failed to read /proc/%d/cmdline",
			pid,
		)
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return linuxApplicationNameByPID(pid)
	}

	return filepath.Base(parts[0]), nil
}

func x11Monitors() ([]x11Monitor, error) {
	display, err := x11OpenDisplay()
	if err != nil {
		return nil, err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var count C.int
	raw := C.neru_x11_get_monitors(display, &count)
	if raw == nil || count == 0 {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"failed to enumerate X11 monitors via XRandR",
		)
	}
	defer C.neru_x11_free_monitors(raw, count) //nolint:nlreturn

	monitors := make([]x11Monitor, 0, int(count))
	rawSlice := unsafe.Slice(raw, int(count))
	for _, monitor := range rawSlice {
		name := ""
		if monitor.name != nil {
			name = C.GoString(monitor.name)
		}
		if name == "" {
			name = fmt.Sprintf("monitor-%d", len(monitors)+1)
		}

		monitors = append(monitors, x11Monitor{
			Name: name,
			Bounds: image.Rect(
				int(monitor.x),
				int(monitor.y),
				int(monitor.x+monitor.width),
				int(monitor.y+monitor.height),
			),
			Primary: monitor.primary != 0,
		})
	}

	return monitors, nil
}

func x11ActiveScreenBounds() (image.Rectangle, error) {
	monitors, err := x11Monitors()
	if err != nil {
		return image.Rectangle{}, err
	}

	cursor, err := x11CursorPosition()
	if err != nil {
		return image.Rectangle{}, err
	}

	for _, monitor := range monitors {
		if cursor.In(monitor.Bounds) {
			return monitor.Bounds, nil
		}
	}

	for _, monitor := range monitors {
		if monitor.Primary {
			return monitor.Bounds, nil
		}
	}

	return monitors[0].Bounds, nil
}

func x11ScreenBoundsByName(name string) (image.Rectangle, bool, error) {
	monitors, err := x11Monitors()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	for _, monitor := range monitors {
		if strings.EqualFold(monitor.Name, name) {
			return monitor.Bounds, true, nil
		}
	}

	return image.Rectangle{}, false, nil
}

func x11ScreenNames() ([]string, error) {
	monitors, err := x11Monitors()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(monitors))
	for _, monitor := range monitors {
		names = append(names, monitor.Name)
	}

	return names, nil
}
