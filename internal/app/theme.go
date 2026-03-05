package app

import (
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
)

// bridgeThemeProvider implements config.ThemeProvider using the native bridge.
type bridgeThemeProvider struct{}

// IsDarkMode returns true if macOS Dark Mode is currently active.
func (b *bridgeThemeProvider) IsDarkMode() bool {
	return bridge.IsDarkMode()
}

// defaultThemeProvider is the shared instance used throughout the app.
var defaultThemeProvider = &bridgeThemeProvider{}
