//go:build darwin

package darwin

/*
#include "alert.h"
*/
import "C"

import "unsafe"

// ConfigOnboardingChoice represents the user's choice in the config onboarding alert.
type ConfigOnboardingChoice int

// ConfigValidationChoice represents the user's choice in the config validation error alert.
type ConfigValidationChoice int

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
)

// ShowConfigValidationError displays a native macOS alert for configuration validation errors.
func ShowConfigValidationError(errorMessage, configPath string) ConfigValidationChoice {
	cError := C.CString(errorMessage)
	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cError)) //nolint:nlreturn
	defer C.free(unsafe.Pointer(cPath))  //nolint:nlreturn

	return ConfigValidationChoice(C.showConfigValidationErrorAlert(cError, cPath))
}

// ShowNotification displays a native macOS notification with a title and message.
func ShowNotification(title, message string) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cMessage)) //nolint:nlreturn

	C.showNotification(cTitle, cMessage)
}

// ShowConfigOnboardingAlert displays a native macOS alert for new users without a config file.
func ShowConfigOnboardingAlert(configPath string) ConfigOnboardingChoice {
	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cPath)) //nolint:nlreturn

	return ConfigOnboardingChoice(C.showConfigOnboardingAlert(cPath))
}
