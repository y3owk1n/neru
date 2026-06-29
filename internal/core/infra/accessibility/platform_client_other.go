//go:build !linux

// internal/core/infra/accessibility/platform_client_other.go
// Selects the default AXClient on non-Linux platforms (macOS/Windows).
// Does not implement accessibility logic itself; just picks the client.

package accessibility

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// NewPlatformAXClient returns the default infrastructure client.
func NewPlatformAXClient(logger *zap.Logger, configProvider config.Provider) AXClient {
	return NewInfraAXClient(logger, configProvider)
}
