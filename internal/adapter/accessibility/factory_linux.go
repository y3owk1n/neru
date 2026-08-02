//go:build linux

package accessibility

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/atspi"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/config"
)

// NewPlatformAXClient returns the AT-SPI-backed client on Linux.
//
// AT-SPI activation is lazy — it happens on the first hints request — so the
// caller does not have to know at construction whether hints are enabled.
func NewPlatformAXClient(logger *zap.Logger, configProvider config.Provider) ax.Client {
	return atspi.New(logger, configProvider)
}
