//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// CheckAccessibilityPermissions verifies that the application has been granted
// accessibility permissions in System Preferences > Privacy & Security > Accessibility.
func CheckAccessibilityPermissions() bool {
	return C.NeruCheckAccessibilityPermissions() != 0
}

// FocusedApplicationPID returns the PID of the currently focused application.
func FocusedApplicationPID() (int, error) {
	ref := C.NeruGetFocusedApplication()
	if ref == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer C.NeruReleaseElement(ref)

	info := C.NeruGetElementInfo(ref)
	if info == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get element info")
	}
	defer C.NeruFreeElementInfo(info)

	return int(info.pid), nil
}

// ApplicationNameByPID returns the name of the application with the given PID.
func ApplicationNameByPID(pid int) (string, error) {
	ref := C.NeruGetApplicationByPID(C.int(pid))
	if ref == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application for PID %d",
			pid,
		)
	}
	defer C.NeruReleaseElement(ref)

	cName := C.NeruGetApplicationName(ref)
	if cName == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application name for PID %d",
			pid,
		)
	}
	defer C.NeruFreeString(cName)

	return C.GoString(cName), nil
}

// ApplicationBundleIDByPID returns the bundle ID of the application with the given PID.
func ApplicationBundleIDByPID(pid int) (string, error) {
	ref := C.NeruGetApplicationByPID(C.int(pid))
	if ref == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application for PID %d",
			pid,
		)
	}
	defer C.NeruReleaseElement(ref)

	cBundleID := C.NeruGetBundleIdentifier(ref)
	if cBundleID == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get bundle ID for PID %d",
			pid,
		)
	}
	defer C.NeruFreeString(cBundleID)

	return C.GoString(cBundleID), nil
}

// SetApplicationAttribute sets an attribute on an application by its PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr))

	val := 0
	if value {
		val = 1
	}

	return C.NeruSetApplicationAttribute(C.int(pid), cAttr, C.int(val)) != 0
}
