//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst xrandr
#include "x11_system.h"
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
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

func linuxApplicationNameByPID(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
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
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
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
