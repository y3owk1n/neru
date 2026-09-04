//go:build windows

package platform

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewSystemPort returns a Windows SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return windows.NewSystemAdapter(), nil
}

// NewFontResolver returns a Windows-backed FontResolver that maps generic
// aliases to native Windows font families and checks the rest against the
// fonts GDI has installed.
func NewFontResolver() ports.FontResolver {
	return windows.NewFontResolver()
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
