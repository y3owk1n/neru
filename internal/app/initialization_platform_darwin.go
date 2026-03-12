//go:build darwin

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// initializePlatformLogger sets up the platform-specific logger.
func initializePlatformLogger(logger *zap.Logger) {
	darwin.InitializeLogger(logger)
}
