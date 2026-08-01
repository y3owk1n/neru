// internal/core/infra/platform/factory.go
// Cross-platform constants and errors shared by the build-tagged factory files.
// The package comment lives in doc.go.

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

	ConfigValidationOK       = 1
	ConfigValidationCopyPath = 2

	AccessibilityPermissionStartupGranted = 1
	AccessibilityPermissionStartupQuit    = 2

	ScreenCapturePermissionStartupGranted = 1
	ScreenCapturePermissionStartupCancel  = 2
	ScreenCapturePermissionStartupQuit    = 3
)
