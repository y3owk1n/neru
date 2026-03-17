//go:build windows

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/windows"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ConfigOnboardingChoice constants represent user choices in the config onboarding alert.
const (
	ConfigOnboardingCreate   = 1
	ConfigOnboardingDefaults = 2
	ConfigOnboardingQuit     = 3
)

// NewSystemPort returns a Windows SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return windows.NewSystemAdapter(), nil
}

// ShowConfigOnboardingAlert is a stub on Windows.
func ShowConfigOnboardingAlert(_ string) int {
	return 0
}
