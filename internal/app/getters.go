package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/event"
	"github.com/y3owk1n/neru/internal/ports"
)

// configSnapshot returns the current config pointer under a read lock.
// Callers should use the returned pointer for all reads within a single
// logical operation to avoid seeing a partially-updated config.
func (a *App) configSnapshot() *config.Config {
	a.configMu.RLock()
	cfg := a.config
	a.configMu.RUnlock()

	return cfg
}

// SetEnabled pauses or resumes the application. Pausing exits the open mode
// and unregisters the global hotkeys now, not at the next application switch.
// Resuming registers them again now.
func (a *App) SetEnabled(enabled bool) {
	a.enabledMu.Lock()
	defer a.enabledMu.Unlock()

	a.setEnabledLocked(enabled)
}

// IsEnabled returns the enabled state of the application.
func (a *App) IsEnabled() bool {
	return a.appState.IsEnabled()
}

// ToggleEnabled atomically toggles the enabled state, as SetEnabled does. The
// read and the write are one step because every writer holds enabledMu.
func (a *App) ToggleEnabled() {
	a.enabledMu.Lock()
	defer a.enabledMu.Unlock()

	a.setEnabledLocked(!a.appState.IsEnabled())
}

// setEnabledLocked applies a pause or resume and publishes it when it changes
// anything. Caller holds enabledMu.
//
// A resume is published before it takes effect and a pause after, so every
// mode event falls between the two: nothing can enter a mode until the state
// flips, and a pause closes the open mode before it reports itself.
func (a *App) setEnabledLocked(enabled bool) {
	changed := a.appState.IsEnabled() != enabled

	if changed && enabled {
		a.events.Publish(event.Event{Name: event.Enable})
	}

	a.appState.SetEnabled(enabled)
	a.applyEnabled(enabled)

	if changed && !enabled {
		a.events.Publish(event.Event{Name: event.Disable})
	}
}

// Events is the bus every lifecycle event is published on.
func (a *App) Events() *event.Bus {
	return a.events
}

// applyEnabled tells the mode handler and the hotkey binder about an
// enabled-state change. Call it from unlocked context, because ExitMode takes
// the handler's lock.
func (a *App) applyEnabled(enabled bool) {
	if !enabled && a.modes != nil {
		a.modes.ExitMode()
	}

	if a.hotkeys != nil {
		a.hotkeys.RefreshFor("")
	}
}

// HintsEnabled returns true if hints are enabled.
func (a *App) HintsEnabled() bool {
	cfg := a.configSnapshot()

	return cfg != nil && cfg.Hints.Enabled
}

// GridEnabled returns true if grid is enabled.
func (a *App) GridEnabled() bool {
	cfg := a.configSnapshot()

	return cfg != nil && cfg.Grid.Enabled
}

// RecursiveGridEnabled returns true if recursive-grid is enabled.
func (a *App) RecursiveGridEnabled() bool {
	cfg := a.configSnapshot()

	return cfg != nil && cfg.RecursiveGrid.Enabled
}

// BisectEnabled returns true if bisect is enabled.
func (a *App) BisectEnabled() bool {
	cfg := a.configSnapshot()

	return cfg != nil && cfg.Bisect.Enabled
}

// Config returns the application configuration.
func (a *App) Config() *config.Config {
	return a.configSnapshot()
}

// Logger returns the application logger.
func (a *App) Logger() *zap.Logger {
	return a.logger
}

// HintsContext returns the hints context.
func (a *App) HintsContext() *hints.Context {
	if a.hintsComponent == nil {
		return nil
	}

	return a.hintsComponent.Context
}

// GetConfigPath returns the config path.
func (a *App) GetConfigPath() string {
	a.configMu.RLock()
	p := a.ConfigPath
	a.configMu.RUnlock()

	return p
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
	return a.appState.OnEnabledStateChanged(callback)
}

// OffEnabledStateChanged unsubscribes a callback by ID.
func (a *App) OffEnabledStateChanged(id uint64) {
	a.appState.OffEnabledStateChanged(id)
}

// IsOverlayHiddenForScreenShare returns whether the overlay is hidden from screen sharing.
func (a *App) IsOverlayHiddenForScreenShare() bool {
	return a.appState.IsHiddenForScreenShare()
}

// SetOverlayHiddenForScreenShare sets whether the overlay should be hidden from screen sharing.
func (a *App) SetOverlayHiddenForScreenShare(hide bool) {
	a.appState.SetHiddenForScreenShare(hide)
}

// ToggleOverlayHiddenForScreenShare atomically toggles the screen share hidden state.
func (a *App) ToggleOverlayHiddenForScreenShare() bool {
	return a.appState.ToggleHiddenForScreenShare()
}

// OnScreenShareStateChanged registers a callback for when the screen share state changes.
func (a *App) OnScreenShareStateChanged(callback func(bool)) uint64 {
	return a.appState.OnScreenShareStateChanged(callback)
}

// OffScreenShareStateChanged unsubscribes a callback by ID.
func (a *App) OffScreenShareStateChanged(id uint64) {
	a.appState.OffScreenShareStateChanged(id)
}

// IsScrollInverted returns whether scroll direction inversion is enabled.
func (a *App) IsScrollInverted() bool {
	return a.appState.IsScrollInverted()
}

// SetScrollInverted sets whether scroll direction inversion is enabled.
func (a *App) SetScrollInverted(inverted bool) {
	a.appState.SetScrollInverted(inverted)
	a.syncScrollInvertToService(inverted)
}

// ToggleScrollInvert toggles the scroll direction inversion and returns the new state.
func (a *App) ToggleScrollInvert() bool {
	newState := a.appState.ToggleScrollInverted()
	a.syncScrollInvertToService(newState)

	return newState
}

// OnScrollInvertStateChanged registers a callback for when the scroll invert state changes.
func (a *App) OnScrollInvertStateChanged(callback func(bool)) uint64 {
	return a.appState.OnScrollInvertStateChanged(callback)
}

// OffScrollInvertStateChanged unsubscribes a callback by ID.
func (a *App) OffScrollInvertStateChanged(id uint64) {
	a.appState.OffScrollInvertStateChanged(id)
}

// syncScrollInvertToService syncs the scroll invert state from AppState to the scroll service.
func (a *App) syncScrollInvertToService(inverted bool) {
	if a.scrollService != nil {
		a.scrollService.SetInvertScroll(inverted)
	}
}
