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
// hintsEnabled is only meaningful on Linux; on other platforms it is ignored.
func NewPlatformAXClient(logger *zap.Logger, configProvider config.Provider, _ bool) AXClient {
	return NewInfraAXClient(logger, configProvider)
}
