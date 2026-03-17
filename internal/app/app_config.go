package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	infra "github.com/y3owk1n/neru/internal/core/infra/accessibility"
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
		// Restore hotkeys with the current (unchanged) config so the app
		// does not remain in a degraded state after a failed reload.
		// Use registerHotkeys directly instead of refreshHotkeysForAppOrCurrent
		// because the latter depends on FocusedAppBundleID which can fail,
		// leaving the app without hotkeys.
		//
		// Guard with HotkeysRegistered() because the blocking native alert
		// shown by ReloadWithAppContext can trigger a focus-change notification
		// (NSWorkspaceDidActivateApplicationNotification), which causes the
		// app observer to call refreshHotkeysForAppOrCurrent and re-register
		// hotkeys before this error path runs. Without this check we would
		// attempt to register duplicate hotkeys, producing HOTKEY_REGISTER_FAILED
		// errors for every binding.
		if a.appState.IsEnabled() && !a.appState.HotkeysRegistered() {
			a.registerHotkeys()
			a.appState.SetHotkeysRegistered(true)
			a.logger.Debug("Hotkeys restored after failed config reload")
		}

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
	if loadResult.Config.Hints.Enabled {
		a.logger.Info("Updating clickable roles",
			zap.Int("count", len(loadResult.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(loadResult.Config.Hints.ClickableRoles, a.logger)
	}
}

// reconfigureAfterUpdate reconfigures components and services after config update.
func (a *App) reconfigureAfterUpdate(loadResult *config.LoadResult) {
	// Update the config pointer under configMu so that concurrent readers
	// (e.g. screen-change handlers, theme observer) see a consistent value.
	a.configMu.Lock()
	a.config = loadResult.Config
	a.ConfigPath = loadResult.ConfigPath
	a.configMu.Unlock()
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

	if a.stickyIndicatorComponent != nil {
		a.stickyIndicatorComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.recursiveGridComponent != nil {
		a.recursiveGridComponent.UpdateConfig(loadResult.Config, a.logger)
	}

	if a.hintService != nil {
		a.hintService.UpdateConfig(loadResult.Config.Hints)

		// Re-create the hint generator if hint_characters changed
		newGen, genErr := domainHint.NewAlphabetGenerator(loadResult.Config.Hints.HintCharacters)
		if genErr != nil {
			a.logger.Error("Failed to create hint generator during reload",
				zap.Error(genErr))
		} else {
			a.hintService.UpdateGenerator(context.Background(), newGen)
		}
	}

	if a.scrollService != nil {
		a.scrollService.UpdateConfig(loadResult.Config.Scroll)
	}

	if a.actionService != nil {
		a.actionService.UpdateConfig(loadResult.Config.Action)
	}

	if a.modes != nil {
		a.modes.UpdateConfig(loadResult.Config)
	}

	if a.ipcController != nil {
		a.ipcController.UpdateConfig(loadResult.Config)
	}

	// Sync hide_overlay_in_screen_share config if it changed
	if a.appState.IsHiddenForScreenShare() != loadResult.Config.General.HideOverlayInScreenShare {
		a.appState.SetHiddenForScreenShare(loadResult.Config.General.HideOverlayInScreenShare)
	}

	a.refreshHotkeysForAppOrCurrent("")
}
