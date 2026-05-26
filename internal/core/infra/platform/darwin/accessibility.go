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
	return C.NeruCheckAccessibilityPermissions() != 0
}

// RequestAccessibilityPermissions asks macOS to start the accessibility
// permission flow and returns whether permission is granted afterward.
func RequestAccessibilityPermissions() bool {
	return C.NeruRequestAccessibilityPermissions() != 0
}

// FocusedApplicationPID returns the PID of the currently focused application.
func FocusedApplicationPID() (int, error) {
	ref := C.NeruGetFocusedApplication()
	if ref == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer C.NeruReleaseElement(ref) //nolint:nlreturn

	info := C.NeruGetElementInfo(ref) //nolint:nlreturn
	if info == nil {
		return 0, derrors.New(derrors.CodeAccessibilityFailed, "failed to get element info")
	}
	defer C.NeruFreeElementInfo(info) //nolint:nlreturn

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
	defer C.NeruReleaseElement(ref) //nolint:nlreturn

	cName := C.NeruGetApplicationName(ref) //nolint:nlreturn
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
	defer C.NeruReleaseElement(ref) //nolint:nlreturn

	cBundleID := C.NeruGetBundleIdentifier(ref) //nolint:nlreturn
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

// HasClickAction checks if the accessibility element has a click action.
// Uses default assumptions for pre-fetched attributes (not hidden, visible, enabled).
func HasClickAction(element unsafe.Pointer) bool {
	if element == nil {
		return false
	}

	clickable := C.NeruHasClickAction(
		element,
		true,  // skipVisCheck: no pre-computed center available in this simplified wrapper
		false, // preHidden
		true,  // preVisible
		true,  // preEnabled
		true,  // hasEnabledAttr
		nil,   // preRole
		false, // preIsWidget
		0,     // centerX
		0,     // centerY
		false, // preHasPressAction
		false, // preHasShowMenuAction
		false, //nolint:nlreturn
	) != 0

	return clickable
}

// SetApplicationAttribute sets an attribute on an application by its PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr)) //nolint:nlreturn

	val := 0
	if value {
		val = 1
	}

	result := C.NeruSetApplicationAttribute(C.int(pid), cAttr, C.int(val)) != 0

	return result
}
