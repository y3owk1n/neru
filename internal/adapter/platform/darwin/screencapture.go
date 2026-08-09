//go:build darwin

package darwin

/*
#include "screencapture.h"
*/
import "C"

// Return values of ShowScreenCapturePermissionAlert, matching the button
// indices the native NSAlert reports. SystemAdapter.RequestScreenCapturePermission
// maps them onto ports.ScreenCaptureConsent, matching Quit and Cancel and
// treating everything else as granted — so screenCaptureAlertGranted has no
// call site and is kept to document the native button order the other two
// index into.
const (
	screenCaptureAlertGranted = 1
	screenCaptureAlertCancel  = 2
	screenCaptureAlertQuit    = 3
)

// CheckScreenCapturePermissions checks if the application has screen recording permission.
func CheckScreenCapturePermissions() bool {
	return C.NeruCheckScreenCapturePermissions() != 0
}

// RequestScreenCapturePermissions requests screen recording permission from macOS.
func RequestScreenCapturePermissions() bool {
	return C.NeruRequestScreenCapturePermissions() != 0
}

// ShowScreenCapturePermissionAlert displays the macOS screen recording permission guidance.
func ShowScreenCapturePermissionAlert() int {
	return int(C.NeruShowScreenCapturePermissionAlert())
}
