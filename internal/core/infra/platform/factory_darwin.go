//go:build darwin

package platform

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewSystemPort returns a macOS SystemPort implementation.
func NewSystemPort() (ports.SystemPort, error) {
	return darwin.NewSystemAdapter(), nil
}
