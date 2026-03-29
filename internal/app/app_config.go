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
	a.prepareForConfigUpdate()

	loadResult, err := a.configService.ReloadWithAppContext(
		ctx,
		configPath,
		a.logger,
	)
	if err != nil {
		a.restoreHotkeysAfterFailedReload()

		return err
	}

	a.applyAppSpecificConfigUpdates(loadResult)
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
	a.updateConfigSnapshot(loadResult)
	a.reconfigureRuntimeFromConfig(loadResult.Config)
	a.refreshHotkeysForAppOrCurrent("")
}

func (a *App) restoreHotkeysAfterFailedReload() {
	// Restore hotkeys with the current (unchanged) config so the app
	// does not remain in a degraded state after a failed reload.
	// Use registerHotkeys directly instead of refreshHotkeysForAppOrCurrent
	// because the latter depends on FocusedAppBundleID which can fail,
	// leaving the app without hotkeys.
	//
	// Guard with HotkeysRegistered() because the blocking native alert
	// shown by ReloadWithAppContext can trigger a focus-change notification,
	// which may already have re-registered hotkeys on another path.
	if a.appState.IsEnabled() && !a.appState.HotkeysRegistered() {
		a.registerHotkeys()
		a.appState.SetHotkeysRegistered(true)
		a.logger.Debug("Hotkeys restored after failed config reload")
	}
}

func (a *App) updateConfigSnapshot(loadResult *config.LoadResult) {
	a.configMu.Lock()
	a.config = loadResult.Config
	a.ConfigPath = loadResult.ConfigPath
	a.configMu.Unlock()
}

func (a *App) reconfigureRuntimeFromConfig(cfg *config.Config) {
	a.configureEventTapHotkeys(cfg, a.logger)
	a.updateComponentConfigs(cfg)
	a.updateServiceConfigs(cfg)
	a.updateControllerConfigs(cfg)
	a.syncScreenShareConfig(cfg)
}

func (a *App) updateComponentConfigs(cfg *config.Config) {
	if a.hintsComponent != nil {
		a.hintsComponent.UpdateConfig(cfg, a.logger)
	}

	if a.gridComponent != nil {
		a.gridComponent.UpdateConfig(cfg, a.logger)
	}

	if a.scrollComponent != nil {
		a.scrollComponent.UpdateConfig(cfg, a.logger)
	}

	if a.modeIndicatorComponent != nil {
		a.modeIndicatorComponent.UpdateConfig(cfg, a.logger)
	}

	if a.stickyIndicatorComponent != nil {
		a.stickyIndicatorComponent.UpdateConfig(cfg, a.logger)
	}

	if a.recursiveGridComponent != nil {
		a.recursiveGridComponent.UpdateConfig(cfg, a.logger)
	}
}

func (a *App) updateServiceConfigs(cfg *config.Config) {
	if a.hintService != nil {
		a.hintService.UpdateConfig(cfg.Hints)

		newGen, genErr := domainHint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
		if genErr != nil {
			a.logger.Error("Failed to create hint generator during reload", zap.Error(genErr))
		} else {
			a.hintService.UpdateGenerator(context.Background(), newGen)
		}
	}

	if a.scrollService != nil {
		a.scrollService.UpdateConfig(cfg.Scroll)
	}
}

func (a *App) updateControllerConfigs(cfg *config.Config) {
	if a.modes != nil {
		a.modes.UpdateConfig(cfg)
	}

	if a.ipcController != nil {
		a.ipcController.UpdateConfig(cfg)
	}
}

func (a *App) syncScreenShareConfig(cfg *config.Config) {
	if a.appState.IsHiddenForScreenShare() != cfg.General.HideOverlayInScreenShare {
		a.appState.SetHiddenForScreenShare(cfg.General.HideOverlayInScreenShare)
	}
}
