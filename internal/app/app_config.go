package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
	infra "github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"go.uber.org/zap"
)

// ReloadConfig reloads the configuration from the specified path.
// If validation fails, shows an alert and keeps the current config.
// Preserves the current app state (enabled/disabled, current mode).
func (a *App) ReloadConfig(ctx context.Context, configPath string) error {
	// Prepare for config update by exiting mode and unregistering hotkeys
	a.prepareForConfigUpdate()

	// Reload config using the service
	loadResult, err := a.configService.ReloadWithAppContext(
		ctx,
		configPath,
		a.logger,
	)
	if err != nil {
		return err
	}

	// Apply app-specific updates
	a.applyAppSpecificConfigUpdates(loadResult)

	// Reconfigure components and services
	a.reconfigureAfterUpdate(loadResult)

	return nil
}

// prepareForConfigUpdate prepares the app for config update by exiting mode and unregistering hotkeys.
func (a *App) prepareForConfigUpdate() {
	if a.appState.CurrentMode() != ModeIdle {
		a.ExitMode()
	}

	if a.appState.HotkeysRegistered() {
		a.logger.Info("Unregistering current hotkeys before reload")
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)
	}
}

// applyAppSpecificConfigUpdates applies app-specific configuration updates.
func (a *App) applyAppSpecificConfigUpdates(loadResult *config.LoadResult) {
	a.config = loadResult.Config
	a.ConfigPath = loadResult.ConfigPath

	if loadResult.Config.Hints.Enabled {
		a.logger.Info("Updating clickable roles",
			zap.Int("count", len(loadResult.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(loadResult.Config.Hints.ClickableRoles, a.logger)
	}
}

// reconfigureAfterUpdate reconfigures components and services after config update.
func (a *App) reconfigureAfterUpdate(loadResult *config.LoadResult) {
	a.configureEventTapHotkeys(loadResult.Config, a.logger)

	if a.hintsComponent != nil {
		a.hintsComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.gridComponent != nil {
		a.gridComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.scrollComponent != nil {
		a.scrollComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.modeIndicatorComponent != nil {
		a.modeIndicatorComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.recursiveGridComponent != nil {
		a.recursiveGridComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.modes != nil {
		a.modes.UpdateConfig(loadResult.Config)
	}

	// Sync hide_overlay_in_screen_share config if it changed
	if a.appState.IsHiddenForScreenShare() != loadResult.Config.General.HideOverlayInScreenShare {
		a.appState.SetHiddenForScreenShare(loadResult.Config.General.HideOverlayInScreenShare)
	}

	a.refreshHotkeysForAppOrCurrent("")
}
