//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst xrandr
#cgo linux LDFLAGS: -lpthread
#include <stdlib.h>
#include "x11_system.h"
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
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

	var posX, posY C.int
	if C.neru_x11_query_pointer(display, &posX, &posY) == 0 { //nolint:nlreturn
		return image.Point{}, derrors.New(
			derrors.CodeActionFailed,
			"failed to query X11 pointer position",
		)
	}

	return image.Point{X: int(posX), Y: int(posY)}, nil
}

func x11MoveCursorToPoint(point image.Point) error {
	display, err := x11OpenDisplay()
	if err != nil {
		return err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	if C.neru_x11_move_pointer(display, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
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
	if C.neru_x11_get_active_window(display, &window) == 0 { //nolint:nlreturn
		return 0, derrors.New(
			derrors.CodeActionFailed,
			"failed to query _NET_ACTIVE_WINDOW on X11",
		)
	}

	var ok C.int
	pid := C.neru_x11_get_window_pid(display, window, &ok) //nolint:nlreturn
	if ok == 0 {
		return 0, derrors.New(
			derrors.CodeActionFailed,
			"failed to query _NET_WM_PID for active X11 window",
		)
	}

	return int(pid), nil
}

// x11FocusedAppID returns the WM_CLASS "class" of the active X11 window, used
// as the per-app bundle identifier (matching accessibility's focused-app
// identity on X11). The bool is false when DISPLAY is unset, no window is
// active, or the window exposes no WM_CLASS.
func x11FocusedAppID() (string, bool) {
	display, err := x11OpenDisplay()
	if err != nil {
		return "", false
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var window C.Window
	if C.neru_x11_get_active_window(display, &window) == 0 { //nolint:nlreturn
		return "", false
	}

	className := C.neru_x11_get_window_class(display, window) //nolint:nlreturn
	if className == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(className)) //nolint:nlreturn

	appID := C.GoString(className)
	if appID == "" {
		return "", false
	}

	return appID, true
}

var (
	x11FocusMonitorMu   sync.Mutex
	x11FocusMonitorInst *C.NeruX11FocusMonitor
)

// x11FocusEventFD lazily starts the X11 focused-window monitor and returns a
// readable fd that becomes ready when the active window changes. ok is false
// when X11 is unavailable (DISPLAY unset, XOpenDisplay failed). The monitor is
// process-global and persists for the daemon lifetime, matching the singleton
// wlroots client; the fd is owned by the monitor and callers must not close it.
func x11FocusEventFD() (int, bool) {
	x11FocusMonitorMu.Lock()
	defer x11FocusMonitorMu.Unlock()

	if x11FocusMonitorInst == nil {
		x11FocusMonitorInst = C.neru_x11_focus_monitor_start()
		if x11FocusMonitorInst == nil {
			return -1, false
		}
	}

	fd := C.neru_x11_focus_monitor_fd(x11FocusMonitorInst) //nolint:nlreturn
	if fd < 0 {
		return -1, false
	}

	return int(fd), true
}

var (
	x11ScreenMonitorMu   sync.Mutex
	x11ScreenMonitorInst *C.NeruX11ScreenMonitor
)

// x11ScreenEventFD lazily starts the X11 RandR screen-change monitor and returns
// a readable fd that becomes ready when the display configuration changes
// (monitors added/removed/resized/moved). ok is false when X11 or the RandR
// extension is unavailable. The monitor is process-global and persists for the
// daemon lifetime, matching x11FocusEventFD; the fd is owned by the monitor and
// callers must not close it.
func x11ScreenEventFD() (int, bool) {
	x11ScreenMonitorMu.Lock()
	defer x11ScreenMonitorMu.Unlock()

	if x11ScreenMonitorInst == nil {
		x11ScreenMonitorInst = C.neru_x11_screen_monitor_start()
		if x11ScreenMonitorInst == nil {
			return -1, false
		}
	}

	fd := C.neru_x11_screen_monitor_fd(x11ScreenMonitorInst) //nolint:nlreturn
	if fd < 0 {
		return -1, false
	}

	return int(fd), true
}

// x11FocusedWindowBounds returns the global bounds of the currently focused
// window via _NET_ACTIVE_WINDOW. found is false (with a nil error) when there is
// no active window or its geometry could not be queried, so callers fall back to
// the active-screen bounds.
func x11FocusedWindowBounds() (image.Rectangle, bool, error) {
	display, err := x11OpenDisplay()
	if err != nil {
		return image.Rectangle{}, false, err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var posX, posY, width, height C.int
	found := C.neru_x11_get_focused_window_bounds(
		display, &posX, &posY, &width, &height, //nolint:nlreturn
	)
	if found == 0 {
		return image.Rectangle{}, false, nil
	}

	return image.Rect(
		int(posX),
		int(posY),
		int(posX+width),
		int(posY+height),
	), true, nil
}

func x11Monitors() ([]x11Monitor, error) {
	display, err := x11OpenDisplay()
	if err != nil {
		return nil, err
	}
	defer C.neru_x11_close_display(display) //nolint:nlreturn

	var count C.int
	raw := C.neru_x11_get_monitors(display, &count) //nolint:nlreturn
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
