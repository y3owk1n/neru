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
// AT-SPI activation is now lazy (on first hints request), so the
// caller does not need to pass a hints-enabled flag at construction.
func NewPlatformAXClient(logger *zap.Logger, configProvider config.Provider) AXClient {
	return NewATSPIClient(logger, configProvider)
}
