//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/ports"
)

// newLinuxSystemAdapter builds the adapter for backend and tells it whether
// this session scrolls through the uinput wheel.
func newLinuxSystemAdapter(backend LinuxBackend) *linux.SystemAdapter {
	adapter := linux.NewSystemAdapter(backend.String())
	adapter.SetUinputScroll(IsHyprlandSession())

	return adapter
}

// NewSystemPort returns a Linux SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	switch backend := detectLinuxBackend(); backend {
	case BackendX11, BackendWaylandWlroots, BackendWaylandKDE, BackendWaylandCOSMIC:
		return newLinuxSystemAdapter(backend), nil
	case BackendWaylandGNOME:
		// Mutter's overlay is drawn on Xwayland. Without it every mode would be
		// invisible, so refuse here.
		if !XwaylandAvailable() {
			return nil, unsupportedLinuxBackendError(backend)
		}

		return newLinuxSystemAdapter(backend), nil
	case BackendUnknown, BackendWaylandOther:
		return nil, unsupportedLinuxBackendError(backend)
	default:
		return nil, unsupportedLinuxBackendError(backend)
	}
}

// NewFontResolver returns a Linux-backed FontResolver backed by fontconfig
// (CGO builds) or a no-CGO passthrough that still maps generic aliases.
func NewFontResolver() ports.FontResolver {
	return linux.NewFontResolver()
}

// NewTextMeasurer returns a cairo-backed TextMeasurer (CGO builds) or one that
// reports CodeNotSupported, which leaves callers on their estimate.
func NewTextMeasurer() ports.TextMeasurer {
	return linux.NewTextMeasurer()
}

// ShowConfigOnboardingAlert tells a first-time user that Neru started on
// built-in defaults, and how to get a config file, then answers with that
// choice.
//
// Unlike macOS's modal NSAlert, Linux answers instead of asking, deliberately.
// A dialog needs a toolkit Neru does not link or a helper (zenity, kdialog) a
// minimal session may lack, and blocking startup on an invisible dialog is
// worse than starting. Neru runs on defaults, changes nothing on disk, and
// says what happened and what to type.
func ShowConfigOnboardingAlert(configPath string) int {
	title := "Neru is running on built-in defaults"
	message := "No configuration file at " + configPath +
		". Run `neru config init` to create one."

	// Bounded by the adapter's own notify deadline. These run before the daemon
	// is up, so a wedged session bus costs a moment rather than the launch.
	err := linux.ShowAlert(context.Background(), title, message)
	if err != nil {
		// Onboarding has no other channel. Nothing upstream prints this, so a
		// session with no notification daemon would otherwise learn nothing.
		fmt.Fprintf(os.Stderr, "⚠️  %s.\n%s\n\n", title, message)
	}

	return ConfigOnboardingDefaults
}

// ShowConfigValidationErrorAlert puts a rejected configuration in front of the
// user before Neru exits, then answers as the dismissed macOS dialog does.
//
// It is a notification for the reasons above, in the alert shape that stays on
// screen until dismissed.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	// The launcher already wrote this failure to stderr, so a missing
	// notification daemon loses only the desktop copy.
	_ = linux.ShowAlert(context.Background(), "Neru could not load "+configPath, errorMessage)

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
