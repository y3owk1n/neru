//go:build darwin

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ConfigOnboardingChoice constants represent user choices in the config onboarding alert.
const (
	ConfigOnboardingCreate   = 1
	ConfigOnboardingDefaults = 2
	ConfigOnboardingQuit     = 3
)

// NewSystemPort returns a macOS SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return darwin.NewSystemAdapter(), nil
}

// ShowConfigOnboardingAlert displays a native macOS alert for new users without a config file.
func ShowConfigOnboardingAlert(configPath string) int {
	return int(darwin.ShowConfigOnboardingAlert(configPath))
}
