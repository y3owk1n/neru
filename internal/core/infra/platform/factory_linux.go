//go:build linux

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a Linux SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	switch backend := detectLinuxBackend(); backend {
	case BackendX11, BackendWaylandWlroots:
		return linux.NewSystemAdapter(backend.String()), nil
	case BackendUnknown, BackendWaylandGNOME, BackendWaylandKDE, BackendWaylandOther:
		return nil, unsupportedLinuxBackendError(backend)
	default:
		return nil, unsupportedLinuxBackendError(backend)
	}
}

// ShowConfigOnboardingAlert is a stub on Linux.
func ShowConfigOnboardingAlert(_ string) int {
	return ConfigOnboardingDefaults
}

// ShowConfigValidationErrorAlert is a stub on Linux.
func ShowConfigValidationErrorAlert(_, _ string) int {
	return ConfigValidationOK
}

// CheckAccessibilityPermissions is always true on Linux for startup gating.
func CheckAccessibilityPermissions() bool {
	return true
}

// ShowAccessibilityPermissionStartupAlert is a no-op on Linux.
func ShowAccessibilityPermissionStartupAlert() int {
	return AccessibilityPermissionStartupGranted
}
