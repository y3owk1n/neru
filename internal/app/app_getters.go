package app

import (
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
	"go.uber.org/zap"
)

// SetEnabled sets the enabled state of the application.
func (a *App) SetEnabled(v bool) {
	a.appState.SetEnabled(v)
}

// IsEnabled returns the enabled state of the application.
func (a *App) IsEnabled() bool {
	return a.appState.IsEnabled()
}

// HintsEnabled returns true if hints are enabled.
func (a *App) HintsEnabled() bool {
	return a.config != nil && a.config.Hints.Enabled
}

// GridEnabled returns true if grid is enabled.
func (a *App) GridEnabled() bool {
	return a.config != nil && a.config.Grid.Enabled
}

// Config returns the application configuration.
func (a *App) Config() *config.Config {
	return a.config
}

// Logger returns the application logger.
func (a *App) Logger() *zap.Logger {
	return a.logger
}

// OverlayManager returns the overlay manager.
func (a *App) OverlayManager() OverlayManager {
	return a.overlayManager
}

// HintsContext returns the hints context.
func (a *App) HintsContext() *hints.Context {
	return a.hintsComponent.Context
}

// Renderer returns the overlay renderer.
func (a *App) Renderer() *ui.OverlayRenderer {
	return a.renderer
}

// GetConfigPath returns the config path.
func (a *App) GetConfigPath() string {
	return a.ConfigPath
}

// SetHintOverlayNeedsRefresh sets the hint overlay needs refresh flag.
func (a *App) SetHintOverlayNeedsRefresh(
	value bool,
) {
	a.appState.SetHintOverlayNeedsRefresh(value)
}

// GridContext returns the grid context.
func (a *App) GridContext() *grid.Context { return a.gridComponent.Context }

// ScrollContext returns the scroll context.
func (a *App) ScrollContext() *scroll.Context { return a.scrollComponent.Context }

// EventTap returns the event tap.
func (a *App) EventTap() ports.EventTapPort { return a.eventTap }

// CurrentMode returns the current mode.
func (a *App) CurrentMode() Mode { return a.appState.CurrentMode() }
