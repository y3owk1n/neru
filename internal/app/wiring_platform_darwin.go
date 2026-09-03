//go:build darwin

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	infrasystray "github.com/y3owk1n/neru/internal/adapter/systray"
)

// initializePlatformLogger sets up the platform-specific logger.
func initializePlatformLogger(logger *zap.Logger) {
	darwin.InitializeLogger(logger)
}

// registerSyntheticModifierSink has nothing to wire on macOS: a CGEvent carries
// its own modifier flags, so injecting an action's modifiers never touches the
// keyboard and there is no key event for the tap to mistake for the user's.
func registerSyntheticModifierSink(_ tap.Tap, _ *zap.Logger) {}

// platformQuit triggers the platform-specific quit mechanism.
func platformQuit() {
	infrasystray.Quit()
}
