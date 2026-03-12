// Package platform provides a factory for platform-specific infrastructure components.
package platform

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ErrUnsupportedPlatform is returned when the current platform is not supported.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// NewSystemPort returns a new SystemPort implementation for the current platform.
func NewSystemPort() (ports.SystemPort, error) {
	switch runtime.GOOS {
	case "darwin":
		return darwin.NewSystemAdapter(), nil
	case "linux":
		return linux.NewSystemAdapter(), nil
	case "windows":
		// Windows uses the darwin package stub (stub_windows.go provides no-op implementations).
		return darwin.NewSystemAdapter(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPlatform, runtime.GOOS)
	}
}
