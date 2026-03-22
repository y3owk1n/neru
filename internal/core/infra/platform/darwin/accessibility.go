//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// CheckAccessibilityPermissions verifies that the application has been granted
// accessibility permissions in System Preferences > Privacy & Security > Accessibility.
func CheckAccessibilityPermissions() bool {
	return C.checkAccessibilityPermissions() != 0
}

// FocusedApplicationPID returns the PID of the currently focused application.
func FocusedApplicationPID() (int, error) {
	ref := C.getFocusedApplication()
	if ref == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer C.releaseElement(ref) //nolint:nlreturn

	info := C.getElementInfo(ref) //nolint:nlreturn
	if info == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get element info")
	}
	defer C.freeElementInfo(info) //nolint:nlreturn

	return int(info.pid), nil
}

// ApplicationNameByPID returns the name of the application with the given PID.
func ApplicationNameByPID(pid int) (string, error) {
	ref := C.getApplicationByPID(C.int(pid))
	if ref == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application for PID %d",
			pid,
		)
	}
	defer C.releaseElement(ref) //nolint:nlreturn

	cName := C.getApplicationName(ref) //nolint:nlreturn
	if cName == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application name for PID %d",
			pid,
		)
	}
	defer C.freeString(cName) //nolint:nlreturn
	return C.GoString(cName), nil
}

// ApplicationBundleIDByPID returns the bundle ID of the application with the given PID.
func ApplicationBundleIDByPID(pid int) (string, error) {
	ref := C.getApplicationByPID(C.int(pid))
	if ref == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get application for PID %d",
			pid,
		)
	}
	defer C.releaseElement(ref) //nolint:nlreturn

	cBundleID := C.getBundleIdentifier(ref) //nolint:nlreturn
	if cBundleID == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"failed to get bundle ID for PID %d",
			pid,
		)
	}
	defer C.freeString(cBundleID) //nolint:nlreturn
	return C.GoString(cBundleID), nil
}

// HasClickAction checks if the accessibility element has a click action.
func HasClickAction(element unsafe.Pointer) bool {
	if element == nil {
		return false
	}

	result := C.hasClickAction(element) != 0 //nolint:nlreturn

	return result
}

// SetApplicationAttribute sets an attribute on an application by its PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr)) //nolint:nlreturn

	val := 0
	if value {
		val = 1
	}

	result := C.setApplicationAttribute(C.int(pid), cAttr, C.int(val)) != 0

	return result
}
