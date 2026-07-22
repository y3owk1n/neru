package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	infra "github.com/y3owk1n/neru/internal/core/infra/accessibility"
)

// SetConfigField applies a single runtime config field change with full
// app-level reconfiguration (component updates, hotkey re-registration, etc.).
// This mirrors the reload path but operates on the in-memory config rather
// than re-reading from disk.
func (a *App) SetConfigField(ctx context.Context, key, value string) error {
	a.prepareForConfigUpdate()

	// Deep copy the current config so we only mutate the new copy.
	newCfg, err := config.DeepCopyConfig(a.config)
	if err != nil {
		a.restoreHotkeysAfterFailedReload()

		return derrors.Wrap(err, derrors.CodeSerializationFailed, "deep copy config")
	}

	// Apply the field change to the copy.
	setErr := config.SetField(newCfg, key, value)
	if setErr != nil {
		a.restoreHotkeysAfterFailedReload()

		return setErr
	}

	// Validate the new config.
	valErr := newCfg.Validate()
	if valErr != nil {
		a.restoreHotkeysAfterFailedReload()

		return derrors.Wrap(valErr, derrors.CodeInvalidConfig, "config-set validation")
	}

	// Update the config service (notifies watchers with the new config).
	updateErr := a.configService.Update(newCfg)
	if updateErr != nil {
		a.restoreHotkeysAfterFailedReload()

		return updateErr
	}

	// Build a LoadResult for the reconfiguration helpers.
	loadResult := &config.LoadResult{
		Config:     newCfg,
		ConfigPath: a.ConfigPath,
	}

	a.applyAppSpecificConfigUpdates(loadResult)
	a.reconfigureAfterUpdate(loadResult)

	// Persist the change to the override file so it survives restarts.
	persistErr := a.configService.SaveOverrideField(key, value)
	if persistErr != nil {
		a.logger.Warn("Failed to persist config override",
			zap.String("key", key),
			zap.Error(persistErr))
	}

	a.logger.Info("Config field updated at runtime",
		zap.String("key", key),
		zap.String("value", value),
	)

	return nil
}

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

	// On Linux, verify the hotkey listener started correctly after reload.
	a.schedulePostReloadVerification()

	return nil
}

// prepareForConfigUpdate prepares the app for config update by exiting mode and unregistering hotkeys.
func (a *App) prepareForConfigUpdate() {
	if a.appState.CurrentMode() != ModeIdle {
		a.ExitMode()
	}

	a.hotkeyRegistrationMu.Lock()
	defer a.hotkeyRegistrationMu.Unlock()

	if a.appState.HotkeysRegistered() {
		a.logger.Debug("Unregistering current hotkeys before reload")
		a.stopAllHotkeyRepeats()
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)
	}
}

// applyAppSpecificConfigUpdates applies app-specific configuration updates.
func (a *App) applyAppSpecificConfigUpdates(loadResult *config.LoadResult) {
	if loadResult.Config.Hints.Enabled {
		a.logger.Debug("Updating clickable roles",
			zap.Int("count", len(loadResult.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(loadResult.Config.Hints.ClickableRoles, a.logger)
	}
}

// reconfigureAfterUpdate reconfigures components and services after config update.
func (a *App) reconfigureAfterUpdate(loadResult *config.LoadResult) {
	a.updateConfigSnapshot(loadResult)
	a.reconfigureRuntimeFromConfig(loadResult.Config)

	// An activation refresh between prepareForConfigUpdate and here may
	// have re-registered with the old config for the current bundle,
	// causing refreshHotkeysForAppOrCurrent to skip because the bundle
	// hasn't changed.  Force clean registration with the new config.
	a.hotkeyRegistrationMu.Lock()
	if a.appState.HotkeysRegistered() {
		a.stopAllHotkeyRepeats()
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)
	}
	a.hotkeyRegistrationMu.Unlock()

	a.refreshHotkeysForAppOrCurrent("")
}

func (a *App) restoreHotkeysAfterFailedReload() {
	a.hotkeyRegistrationMu.Lock()
	defer a.hotkeyRegistrationMu.Unlock()

	if a.appState.IsEnabled() && !a.appState.HotkeysRegistered() {
		// Query the currently focused app so we register the right
		// [[app_configs]] overrides.  Focus can change while the
		// blocking reload-alert dialog is up, making the cached
		// currentHotkeyBundleID stale.
		bundleID, err := a.actionService.FocusedAppBundleID(a.ctx)
		if err != nil {
			bundleID = ""
		}

		a.registerHotkeys(bundleID)
		a.appState.SetHotkeysRegistered(true)
		a.currentHotkeyBundleID = bundleID
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
	a.syncScrollInvertConfig(cfg)
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

	if a.virtualPointerOverlay != nil {
		a.virtualPointerOverlay.SetConfig(cfg.VirtualPointer)
	}
}

func (a *App) updateServiceConfigs(cfg *config.Config) {
	if a.hintService != nil {
		a.hintService.UpdateConfig(cfg.Hints)

		newGen, genErr := domainHint.NewAlphabetGenerator(
			cfg.Hints.HintCharacters,
			domainHint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
		)
		if genErr != nil {
			a.logger.Error("Failed to create hint generator during reload", zap.Error(genErr))
		} else {
			a.hintService.UpdateGenerator(a.ctx, newGen)
		}

		// Re-register the opposite-direction generator so the per-activation
		// override path keeps working after a config reload.
		registerOppositeLabelDirectionGenerator(a, a.hintService, cfg)
	}

	if a.scrollService != nil {
		a.scrollService.UpdateConfig(cfg.Scroll)
	}

	if a.actionService != nil {
		a.actionService.UpdateConfig(cfg.MouseAction)
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

func (a *App) syncScrollInvertConfig(cfg *config.Config) {
	if a.appState.IsScrollInverted() != cfg.Scroll.InvertScroll {
		a.appState.SetScrollInverted(cfg.Scroll.InvertScroll)
		a.syncScrollInvertToService(cfg.Scroll.InvertScroll)
	}
}

// syncInitialConfigToAppState syncs configuration values to AppState during startup.
// This ensures AppState reflects the config file values before any runtime toggles.
func syncInitialConfigToAppState(app *App) {
	cfg := app.configSnapshot()

	if app.appState.IsHiddenForScreenShare() != cfg.General.HideOverlayInScreenShare {
		app.appState.SetHiddenForScreenShare(cfg.General.HideOverlayInScreenShare)
	}

	if app.appState.IsScrollInverted() != cfg.Scroll.InvertScroll {
		app.appState.SetScrollInverted(cfg.Scroll.InvertScroll)
	}
}
