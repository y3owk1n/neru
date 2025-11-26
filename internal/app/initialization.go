package app

import (
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// initializeLogger initializes the application logger with the given configuration.
func initializeLogger(config *config.Config) (*zap.Logger, error) {
	initConfigErr := logger.Init(
		config.Logging.LogLevel,
		config.Logging.LogFile,
		config.Logging.StructuredLogging,
		config.Logging.DisableFileLogging,
		config.Logging.MaxFileSize,
		config.Logging.MaxBackups,
		config.Logging.MaxAge,
	)
	if initConfigErr != nil {
		return nil, derrors.Wrap(initConfigErr, derrors.CodeInternal, "failed to initialize logger")
	}

	logger := logger.Get()
	bridge.InitializeLogger(logger)

	return logger, nil
}

// initializeOverlayManager creates and initializes the overlay manager.
func initializeOverlayManager(deps *Deps, logger *zap.Logger) OverlayManager {
	if deps != nil && deps.OverlayManagerFactory != nil {
		return deps.OverlayManagerFactory.New(logger)
	}

	return overlay.Init(logger)
}

// initializeAccessibility checks and configures accessibility permissions and settings.
func initializeAccessibility(cfg *config.Config, logger *zap.Logger) error {
	if cfg.General.AccessibilityCheckOnStart {
		if !accessibility.CheckAccessibilityPermissions() {
			logger.Warn(
				"Accessibility permissions not granted. Please grant permissions in System Settings.",
			)
			logger.Info("⚠️  Neru requires Accessibility permissions to function.")
			logger.Info("Please go to: System Settings → Privacy & Security → Accessibility")
			logger.Info("and enable Neru.")

			return derrors.New(
				derrors.CodeAccessibilityDenied,
				"accessibility permissions not granted - please enable in System Preferences",
			)
		}
	}

	config.SetGlobal(cfg)

	// Apply clickable roles if hints are enabled
	if cfg.Hints.Enabled {
		logger.Info("Applying clickable roles",
			zap.Int("count", len(cfg.Hints.ClickableRoles)),
			zap.Strings("roles", cfg.Hints.ClickableRoles))
		accessibility.SetClickableRoles(cfg.Hints.ClickableRoles)
	}

	return nil
}

// initializeHotkeyService creates the hotkey service, using the provided dependency or creating a new one.
func initializeHotkeyService(deps *Deps, logger *zap.Logger) HotkeyService {
	if deps != nil && deps.Hotkeys != nil {
		return deps.Hotkeys
	}

	hotkeyManager := hotkeys.NewManager(logger)
	hotkeys.SetGlobalManager(hotkeyManager)

	return hotkeyManager
}

func initializeAppWatcher(deps *Deps, logger *zap.Logger) Watcher {
	if deps != nil && deps.WatcherFactory != nil {
		return deps.WatcherFactory.New(logger)
	}

	return appwatcher.NewWatcher(logger)
}

// configureEventTapHotkeys configures the event tap with hotkeys from the configuration.
func (a *App) configureEventTapHotkeys(config *config.Config, logger *zap.Logger) {
	keys := make([]string, 0, len(config.Hotkeys.Bindings))
	for key, value := range config.Hotkeys.Bindings {
		// Skip empty keys or values
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			logger.Warn(
				"Skipping empty hotkey binding",
				zap.String("key", key),
				zap.String("value", value),
			)

			continue
		}

		mode := value
		if parts := strings.Split(value, " "); len(parts) > 0 {
			mode = parts[0]
		}

		if mode == domain.ModeString(domain.ModeHints) && !config.Hints.Enabled {
			continue
		}

		if mode == domain.ModeString(domain.ModeGrid) && !config.Grid.Enabled {
			continue
		}

		keys = append(keys, key)
	}

	// Log if no hotkeys are configured
	if len(keys) == 0 {
		logger.Warn("No hotkeys configured - application will not be activatable via hotkeys")
	} else {
		logger.Info("Registered hotkeys", zap.Int("count", len(keys)))
	}

	a.eventTap.SetHotkeys(keys)
	a.eventTap.Disable()
}

// registerOverlays registers all component overlays with the overlay manager.
func (a *App) registerOverlays() {
	if a.scrollComponent != nil && a.scrollComponent.Overlay != nil {
		a.overlayManager.UseScrollOverlay(a.scrollComponent.Overlay)
	}

	if a.actionComponent != nil && a.actionComponent.Overlay != nil {
		a.overlayManager.UseActionOverlay(a.actionComponent.Overlay)
	}

	if a.hintsComponent != nil && a.hintsComponent.Overlay != nil {
		a.overlayManager.UseHintOverlay(a.hintsComponent.Overlay)
	}

	if a.gridComponent != nil && a.gridComponent.Context != nil &&
		a.gridComponent.Context.GridOverlay() != nil {
		a.overlayManager.UseGridOverlay(*a.gridComponent.Context.GridOverlay())
	}
}
