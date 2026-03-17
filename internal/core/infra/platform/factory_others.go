//go:build !darwin && !linux && !windows

package platform

import (
	"fmt"
	"runtime"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// ConfigOnboardingChoice constants represent user choices in the config onboarding alert.
const (
	ConfigOnboardingCreate   = 1
	ConfigOnboardingDefaults = 2
	ConfigOnboardingQuit     = 3
)

// NewSystemPort returns an error on unsupported platforms.
func NewSystemPort() (ports.SystemPort, error) {
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPlatform, runtime.GOOS)
}

// ShowConfigOnboardingAlert is a stub on non-darwin platforms.
func ShowConfigOnboardingAlert(_ string) int {
	return 0
}
