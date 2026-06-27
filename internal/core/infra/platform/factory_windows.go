//go:build windows

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/windows"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a Windows SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return windows.NewSystemAdapter(), nil
}

// NewFontResolver returns the process-wide no-op FontResolver on Windows.
// Windows builds do not yet have a font resolver; the value is returned
// unchanged so the existing C paths continue to work.
func NewFontResolver() ports.FontResolver {
	return nil
}

// ShowConfigOnboardingAlert displays a native Windows dialog for new users without a config file.
func ShowConfigOnboardingAlert(configPath string) int {
	return windows.ShowConfigOnboardingAlert(configPath)
}

// ShowConfigValidationErrorAlert displays a native Windows dialog for config validation errors.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	return windows.ShowConfigValidationErrorAlert(errorMessage, configPath)
}

// CheckAccessibilityPermissions is always true on Windows for startup gating.
func CheckAccessibilityPermissions() bool {
	return true
}

// ShowAccessibilityPermissionStartupAlert is a no-op on Windows.
func ShowAccessibilityPermissionStartupAlert() int {
	return AccessibilityPermissionStartupGranted
}

// CheckScreenCapturePermissions is always true on Windows.
func CheckScreenCapturePermissions() bool {
	return true
}

// ShowScreenCapturePermissionAlert is a no-op on Windows.
func ShowScreenCapturePermissionAlert() int {
	return ScreenCapturePermissionStartupGranted
}
