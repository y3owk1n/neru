//go:build darwin

package darwin

/*
#include "screencapture.h"
*/
import "C"

// Return values of ShowScreenCapturePermissionAlert, matching the button
// indices the native NSAlert reports. The platform package mirrors these as
// ScreenCapturePermissionStartup* for its startup flow.
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
