//go:build darwin

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a macOS SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return darwin.NewSystemAdapter(), nil
}

// ShowConfigOnboardingAlert displays a native macOS alert for new users without a config file.
func ShowConfigOnboardingAlert(configPath string) int {
	return int(darwin.ShowConfigOnboardingAlert(configPath))
}

// ShowConfigValidationErrorAlert displays a native macOS alert for config validation errors.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	return int(darwin.ShowConfigValidationError(errorMessage, configPath))
}

// CheckAccessibilityPermissions checks macOS accessibility permission without prompting.
func CheckAccessibilityPermissions() bool {
	return darwin.CheckAccessibilityPermissions()
}

// ShowAccessibilityPermissionStartupAlert displays the macOS startup permission guidance.
func ShowAccessibilityPermissionStartupAlert() int {
	return int(darwin.ShowAccessibilityPermissionStartupAlert())
}
