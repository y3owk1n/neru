//go:build darwin

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	infrasystray "github.com/y3owk1n/neru/internal/core/infra/systray"
)

// initializePlatformLogger sets up the platform-specific logger.
func initializePlatformLogger(logger *zap.Logger) {
	darwin.InitializeLogger(logger)
}

// platformQuit triggers the platform-specific quit mechanism.
func platformQuit() {
	infrasystray.Quit()
}
