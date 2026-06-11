package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
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

// handleThemeChange is called when the system appearance changes.
// It refreshes overlay styles that depend on the theme for all active modes.
func (a *App) handleThemeChange(isDark bool) {
	a.configMu.RLock()
	cfg := a.config
	a.configMu.RUnlock()
	a.logger.Info("System theme changed",
		zap.Bool("is_dark", isDark))
	// Invalidate the overlay's native C string caches so the subsequent draw
	// rebuilds them with the new theme-resolved colors.
	if a.hintsComponent != nil && a.hintsComponent.Overlay != nil {
		a.hintsComponent.UpdateConfig(cfg, a.logger)
	}

	if a.gridComponent != nil && a.gridComponent.Overlay != nil {
		a.gridComponent.UpdateConfig(cfg, a.logger)
	}

	if a.recursiveGridComponent != nil {
		a.recursiveGridComponent.UpdateConfig(cfg, a.logger)
	}

	if a.modeIndicatorComponent != nil {
		a.modeIndicatorComponent.UpdateConfig(cfg, a.logger)
	}

	if a.stickyIndicatorComponent != nil {
		a.stickyIndicatorComponent.UpdateConfig(cfg, a.logger)
	}

	// Re-build renderer style with the new theme state, then redraw active mode.
	if a.modes != nil {
		a.modes.UpdateConfig(cfg)

		currentMode := a.appState.CurrentMode()
		switch currentMode {
		case domain.ModeHints:
			a.modes.RefreshHintsForThemeChange()
		case domain.ModeGrid:
			a.modes.RefreshGridForThemeChange()
		case domain.ModeRecursiveGrid:
			a.modes.RefreshRecursiveGridForThemeChange()
		case domain.ModeIdle, domain.ModeScroll:
			// No-op for idle and scroll modes as they don't have theme-dependent persistent overlays
			// that need immediate refresh here. Scroll mode indicator is handled via its own component refresh above.
		}
	}
}
