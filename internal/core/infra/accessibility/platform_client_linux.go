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
func NewPlatformAXClient(logger *zap.Logger, configProvider config.Provider) AXClient {
	return NewATSPIClient(logger, configProvider)
}
