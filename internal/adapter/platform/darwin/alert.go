//go:build darwin

package darwin

/*
#include "alert.h"
*/
import "C"

import "unsafe"

// ConfigOnboardingChoice is the user's choice in the config onboarding alert.
type ConfigOnboardingChoice int

// ConfigValidationChoice is the user's choice in the config validation error alert.
type ConfigValidationChoice int

// AccessibilityPermissionStartupChoice is the user's choice in the startup permission alert.
type AccessibilityPermissionStartupChoice int

const (
	// ConfigOnboardingCreate indicates the user chose to create a config file.
	ConfigOnboardingCreate ConfigOnboardingChoice = 1
	// ConfigOnboardingDefaults indicates the user chose to use default configuration.
	ConfigOnboardingDefaults ConfigOnboardingChoice = 2
	// ConfigOnboardingQuit indicates the user chose to quit the application.
	ConfigOnboardingQuit ConfigOnboardingChoice = 3

	// ConfigValidationOK indicates the user clicked OK.
	ConfigValidationOK ConfigValidationChoice = 1
	// ConfigValidationCopyPath indicates the user clicked Copy Path.
	ConfigValidationCopyPath ConfigValidationChoice = 2

	// AccessibilityPermissionStartupGranted indicates accessibility permission is granted.
	AccessibilityPermissionStartupGranted AccessibilityPermissionStartupChoice = 1
	// AccessibilityPermissionStartupQuit indicates the user chose to quit.
	AccessibilityPermissionStartupQuit AccessibilityPermissionStartupChoice = 2
)

// ShowConfigValidationError displays a native macOS alert for configuration validation errors.
func ShowConfigValidationError(errorMessage, configPath string) ConfigValidationChoice {
	cError := C.CString(errorMessage)
	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cError))
	defer C.free(unsafe.Pointer(cPath))

	return ConfigValidationChoice(C.NeruShowConfigValidationErrorAlert(cError, cPath))
}

// ShowNotification displays a native macOS notification with a title and message.
func ShowNotification(title, message string) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))

	C.NeruShowNotification(cTitle, cMessage)
}

// ShowConfigOnboardingAlert displays a native macOS alert for new users without a config file.
func ShowConfigOnboardingAlert(configPath string) ConfigOnboardingChoice {
	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cPath))

	return ConfigOnboardingChoice(C.NeruShowConfigOnboardingAlert(cPath))
}

// ShowAccessibilityPermissionStartupAlert displays the macOS startup guidance for granting
// accessibility permission.
func ShowAccessibilityPermissionStartupAlert() AccessibilityPermissionStartupChoice {
	return AccessibilityPermissionStartupChoice(C.NeruShowAccessibilityPermissionStartupAlert())
}
