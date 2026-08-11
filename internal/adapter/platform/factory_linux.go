//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewSystemPort returns a Linux SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	switch backend := detectLinuxBackend(); backend {
	case BackendX11, BackendWaylandWlroots, BackendWaylandKDE:
		return linux.NewSystemAdapter(backend.String()), nil
	case BackendUnknown, BackendWaylandGNOME, BackendWaylandOther:
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

// ShowConfigOnboardingAlert tells a first-time user that Neru started on
// built-in defaults, and how to get a config file, then answers with that
// choice.
//
// macOS asks the question in a modal NSAlert and waits — create, use defaults,
// or quit. Linux answers it instead of asking, deliberately. A modal dialog
// here needs a toolkit Neru does not link or a helper (zenity, kdialog) a
// minimal wlroots session need not have, so the honest choices were a prompt
// that may never appear or a message plus a safe default. Blocking startup on
// a dialog nobody can see is worse than starting; a keyboard-driven tool whose
// first act is to seize the session for a question is worse still. So the
// question is answered the way the user could only have answered it after
// being asked — run, on defaults, changing nothing on disk — and they are told
// what happened and what to type.
func ShowConfigOnboardingAlert(configPath string) int {
	title := "Neru is running on built-in defaults"
	message := "No configuration file at " + configPath +
		". Run `neru config init` to create one."

	// Bounded by the adapter's own notify deadline; these run before the daemon
	// is up, so a wedged session bus costs a moment rather than the launch.
	err := linux.ShowAlert(context.Background(), title, message)
	if err != nil {
		// Onboarding has no other channel: nothing upstream prints this, so a
		// session with no notification daemon would otherwise learn nothing.
		fmt.Fprintf(os.Stderr, "⚠️  %s.\n%s\n\n", title, message)
	}

	return ConfigOnboardingDefaults
}

// ShowConfigValidationErrorAlert puts a rejected configuration in front of the
// user before Neru exits, then answers as the dismissed macOS dialog does.
//
// It is a notification rather than a dialog for the reasons above; the alert
// shape it uses stays on screen until dismissed, so a message that arrives
// while the user is looking elsewhere is still there when they look back.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	// The launcher has already written the same failure to stderr, so a
	// missing notification daemon costs the desktop copy rather than the
	// message — nothing to fall back to here.
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
