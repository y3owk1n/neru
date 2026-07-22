//go:build linux

// internal/core/infra/accessibility/platform_client_linux.go
// Selects the Linux AXClient (AT-SPI tree walking + libei input).
// Does not implement accessibility logic itself; just picks the client.

package accessibility

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// NewPlatformAXClient returns the AT-SPI-backed client on Linux.
// When hintsEnabled is false the underlying AT-SPI client skips activating
// the accessibility bus, avoiding unnecessary screen-reader prompts.
func NewPlatformAXClient(
	logger *zap.Logger,
	configProvider config.Provider,
	hintsEnabled bool,
) AXClient {
	return NewATSPIClient(logger, configProvider, hintsEnabled)
}
