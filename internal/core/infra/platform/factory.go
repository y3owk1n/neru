// Package platform provides a factory for platform-specific infrastructure components.
//
// The NewSystemPort function is defined in build-tagged files
// (factory_darwin.go, factory_linux.go, factory_windows.go) so that each
// platform only imports its own adapter package. This avoids pulling in CGo
// dependencies (platform/darwin) on non-macOS builds.
package platform

import (
	"errors"
)

// ErrUnsupportedPlatform is returned when the current platform is not supported.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// ConfigOnboardingChoice constants represent user choices in the config onboarding alert.
const (
	ConfigOnboardingCreate   = 1
	ConfigOnboardingDefaults = 2
	ConfigOnboardingQuit     = 3
)
