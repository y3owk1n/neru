package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
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

// HandleThemeChange is the app's entry point for a system appearance change:
// the platform theme observers call it, and the simulation harness drives it
// the same way.
//
// It is one notification, not a fan-out: the overlay re-resolves every Style
// from the configuration it already holds and invalidates the render
// components' native caches, so the only thing left to do here is redraw
// whichever mode is currently on screen.
func (a *App) HandleThemeChange(isDark bool) {
	a.logger.Info("System theme changed",
		zap.Bool("is_dark", isDark))

	if a.overlayPort != nil {
		a.overlayPort.RefreshStyles()
	}

	if a.modes != nil {
		currentMode := a.appState.CurrentMode()
		switch currentMode {
		case domain.ModeHints:
			a.modes.RefreshHintsForThemeChange()
		case domain.ModeGrid:
			a.modes.RefreshGridForThemeChange()
		case domain.ModeRecursiveGrid:
			a.modes.RefreshRecursiveGridForThemeChange()
		case domain.ModeMonitorSelect:
			a.modes.RefreshMonitorSelectForThemeChange()
		case domain.ModeIdle, domain.ModeScroll:
			// No-op for idle and scroll modes as they don't have theme-dependent persistent overlays
			// that need immediate refresh here. Scroll mode indicator is handled via its own component refresh above.
		}
	}
}
