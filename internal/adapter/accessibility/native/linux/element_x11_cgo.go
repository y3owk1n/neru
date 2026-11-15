//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <stdlib.h>
#include "../../../platform/linux/x11_accessibility.h"
#include "../../../platform/linux/x11_system.h"
*/
import "C"

import (
	"os"
	"unsafe"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// linuxFocusedApplicationIdentity answers which application owns the focused
// X11 window, as a WM_CLASS/pid pair, and ("", 0) when it cannot say. The
// active window and its identity come from the system bridge (x11_system.h),
// not a second copy here (#1499), since that copy let a closed window exit the
// daemon. The bridge's five answers collapse to one on purpose. This signature
// has no error, and its callers re-sample on the next focus event anyway.
func linuxFocusedApplicationIdentity() (string, int) {
	if os.Getenv("DISPLAY") == "" {
		return "", 0
	}

	display := C.neru_ax_open_display()
	if display == nil {
		return "", 0
	}
	defer C.neru_ax_close_display(display)

	var window C.Window
	if C.neru_x11_get_active_window(display, &window) != C.int(C.NERU_X11_ACTIVE_WINDOW_OK) {
		return "", 0
	}

	className := C.neru_x11_get_window_class(display, window)

	bundleID := ""
	if className != nil {
		bundleID = C.GoString(className)
		C.free(unsafe.Pointer(className))
	}

	// The pid answers collapse the same way, since this caller has no error to
	// carry. system_x11_window_pid.go tells them apart for the caller that can
	// act on it.
	var pid C.ulong
	if C.neru_x11_get_window_pid(display, window, &pid) != C.int(C.NERU_X11_WINDOW_PID_OK) {
		pid = 0
	}

	return bundleID, int(pid)
}
