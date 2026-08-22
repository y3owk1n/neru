package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
)

// newConfigService builds the config service from both halves of the load the
// app was constructed with: the configuration to run on, and the one it was
// derived from. Passing only the first leaves the service deriving from its own
// output, which cannot re-infer — so the two travel together from here on.
func (a *App) newConfigService(logger *zap.Logger) *loader.Service {
	return loader.NewService(a.config, a.ConfigPath, logger, a.systemPort).
		WithWritten(a.writtenConfig)
}

// SetConfigField applies a single runtime config field change with full
// app-level reconfiguration (component updates, hotkey re-registration, etc.).
// This mirrors the reload path but operates on the in-memory config rather
// than re-reading from disk.
//
// Derived values come along, because the change is applied to the
// configuration the user wrote and the derivation is run again over the result
// — so `neru config set grid.characters` relabels the grid at once, and
// `neru config set theme.*` recolours at once. Both halves are read from the
// service, which is the source of truth for prior --no-reload changes.
func (a *App) SetConfigField(ctx context.Context, key, value string) error {
	a.prepareForConfigUpdate()

	newCfg, newWritten, err := loader.ApplyFieldChange(a.configService.Written(), key, value)
	if err != nil {
		a.restoreHotkeysAfterFailedReload()

		return err
	}

	valErr := newCfg.Validate()
	if valErr != nil {
		a.restoreHotkeysAfterFailedReload()

		return derrors.Wrap(valErr, derrors.CodeInvalidConfig, "config-set validation")
	}

	updateErr := a.configService.Update(newCfg, newWritten)
	if updateErr != nil {
		a.restoreHotkeysAfterFailedReload()

		return updateErr
	}

	loadResult := &config.LoadResult{
		Config:     newCfg,
		Written:    newWritten,
		ConfigPath: a.ConfigPath,
	}

	a.applyAppSpecificConfigUpdates(loadResult)
	a.reconfigureAfterUpdate(loadResult)

	// Persist the change to the override file so it survives restarts.
	persistErr := a.configService.SaveOverrideField(key, value)
	if persistErr != nil {
		return derrors.Wrap(persistErr, derrors.CodeConfigIOFailed,
			"config set at runtime but failed to persist (the change will not survive a restart)")
	}

	// Only the key: it names a schema field. The value is what the user typed,
	// and the log is not a place config content goes — the same rule the IPC
	// controller states in handleConfigSetInMemory.
	a.logger.Info("Config field updated at runtime",
		zap.String("key", key),
		zap.Int("value_length", len(value)),
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

	a.hotkeys.Unregister()
}

// applyAppSpecificConfigUpdates applies app-specific configuration updates.
func (a *App) applyAppSpecificConfigUpdates(loadResult *config.LoadResult) {
	if loadResult.Config.Hints.Enabled && a.accessibility != nil {
		roles := loadResult.Config.Hints.ResolvedClickableRoles()

		a.logger.Debug("Updating clickable roles",
			zap.Int("configured", len(loadResult.Config.Hints.ClickableRoles)),
			zap.Int("resolved", len(roles)))
		a.accessibility.UpdateClickableRoles(roles)
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
	a.hotkeys.ForceRefresh()
}

func (a *App) restoreHotkeysAfterFailedReload() {
	a.hotkeys.Restore()
}

func (a *App) updateConfigSnapshot(loadResult *config.LoadResult) {
	a.configMu.Lock()
	a.config = loadResult.Config
	a.ConfigPath = loadResult.ConfigPath
	a.configMu.Unlock()
}

func (a *App) reconfigureRuntimeFromConfig(cfg *config.Config) {
	a.configureEventTapHotkeys(cfg, a.logger)

	// One notification reaches every overlay: the overlay re-resolves each
	// Style and hands the new configuration to the components it draws through.
	if a.overlayPort != nil {
		a.overlayPort.ApplyConfig(cfg)
	}

	a.updateServiceConfigs(cfg)
	a.updateControllerConfigs(cfg)
	a.syncScreenShareConfig(cfg)
	a.syncScrollInvertConfig(cfg)
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
// AppState then reflects the config file before any runtime toggle is applied.
func syncInitialConfigToAppState(app *App) {
	cfg := app.configSnapshot()

	if app.appState.IsHiddenForScreenShare() != cfg.General.HideOverlayInScreenShare {
		app.appState.SetHiddenForScreenShare(cfg.General.HideOverlayInScreenShare)
	}

	if app.appState.IsScrollInverted() != cfg.Scroll.InvertScroll {
		app.appState.SetScrollInverted(cfg.Scroll.InvertScroll)
	}
}
