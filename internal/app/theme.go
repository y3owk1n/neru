package app

import (
	"github.com/y3owk1n/neru/internal/core/ports"
)

// bridgeThemeProvider implements config.ThemeProvider using a SystemPort.
type bridgeThemeProvider struct {
	systemPort ports.SystemPort
}

// IsDarkMode returns true if the platform's dark mode is currently active.
func (b *bridgeThemeProvider) IsDarkMode() bool {
	if b.systemPort == nil {
		return false
	}

	return b.systemPort.IsDarkMode()
}

// newThemeProvider creates a new theme provider using the provided system port.
func newThemeProvider(systemPort ports.SystemPort) *bridgeThemeProvider {
	return &bridgeThemeProvider{systemPort: systemPort}
}
