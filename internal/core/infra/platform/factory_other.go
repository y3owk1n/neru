//go:build !darwin && !linux && !windows

package platform

import (
	"fmt"
	"runtime"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns an error on unsupported platforms.
func NewSystemPort() (ports.SystemPort, error) {
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPlatform, runtime.GOOS)
}

// ShowConfigOnboardingAlert is a stub on non-darwin platforms.
func ShowConfigOnboardingAlert(_ string) int {
	return ConfigOnboardingDefaults
}

// ShowConfigValidationErrorAlert is a stub on non-darwin platforms.
func ShowConfigValidationErrorAlert(_, _ string) int {
	return ConfigValidationOK
}

// CheckAccessibilityPermissions is always true on unsupported platforms for startup gating.
func CheckAccessibilityPermissions() bool {
	return true
}

// ShowAccessibilityPermissionStartupAlert is a no-op on unsupported platforms.
func ShowAccessibilityPermissionStartupAlert() int {
	return AccessibilityPermissionStartupGranted
}
