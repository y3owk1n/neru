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

// QuadGridEnabled returns true if quad-grid is enabled.
func (a *App) QuadGridEnabled() bool {
	return a.config != nil && a.config.QuadGrid.Enabled
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
	if a.hintsComponent == nil {
		return nil
	}

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
func (a *App) GridContext() *grid.Context {
	if a.gridComponent == nil {
		return nil
	}

	return a.gridComponent.Context
}

// ScrollContext returns the scroll context.
func (a *App) ScrollContext() *scroll.Context {
	if a.scrollComponent == nil {
		return nil
	}

	return a.scrollComponent.Context
}

// EventTap returns the event tap.
func (a *App) EventTap() ports.EventTapPort { return a.eventTap }

// CurrentMode returns the current mode.
func (a *App) CurrentMode() Mode { return a.appState.CurrentMode() }

// GetSystrayComponent returns the systray component.
func (a *App) GetSystrayComponent() SystrayComponent {
	return a.systrayComponent
}

// OnEnabledStateChanged registers a callback for when the enabled state changes.
// Returns a subscription ID that can be used to unsubscribe later.
func (a *App) OnEnabledStateChanged(callback func(bool)) uint64 {
	// Delegate to appState
	return a.appState.OnEnabledStateChanged(callback)
}

// OffEnabledStateChanged unsubscribes a callback by ID.
func (a *App) OffEnabledStateChanged(id uint64) {
	// Delegate to appState
	a.appState.OffEnabledStateChanged(id)
}

// IsOverlayHiddenForScreenShare returns whether the overlay is hidden from screen sharing.
func (a *App) IsOverlayHiddenForScreenShare() bool {
	return a.appState.IsHiddenForScreenShare()
}

// SetOverlayHiddenForScreenShare sets whether the overlay should be hidden from screen sharing.
func (a *App) SetOverlayHiddenForScreenShare(hide bool) {
	// Update app state (this will trigger callbacks)
	a.appState.SetHiddenForScreenShare(hide)
}

// OnScreenShareStateChanged registers a callback for when the screen share state changes.
func (a *App) OnScreenShareStateChanged(callback func(bool)) uint64 {
	return a.appState.OnScreenShareStateChanged(callback)
}

// OffScreenShareStateChanged unsubscribes a callback by ID.
func (a *App) OffScreenShareStateChanged(id uint64) {
	a.appState.OffScreenShareStateChanged(id)
}
