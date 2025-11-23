package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// initializeLogger initializes the application logger with the given configuration.
func initializeLogger(cfg *config.Config) (*zap.Logger, error) {
	err := logger.Init(
		cfg.Logging.LogLevel,
		cfg.Logging.LogFile,
		cfg.Logging.StructuredLogging,
		cfg.Logging.DisableFileLogging,
		cfg.Logging.MaxFileSize,
		cfg.Logging.MaxBackups,
		cfg.Logging.MaxAge,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	log := logger.Get()
	bridge.InitializeLogger(log)

	return log, nil
}

// initializeOverlayManager creates and initializes the overlay manager.
func initializeOverlayManager(log *zap.Logger) *overlay.Manager {
	return overlay.Init(log)
}

// initializeAccessibility checks and configures accessibility permissions and settings.
func initializeAccessibility(cfg *config.Config, log *zap.Logger) error {
	if cfg.General.AccessibilityCheckOnStart {
		if !accessibility.CheckAccessibilityPermissions() {
			log.Warn(
				"Accessibility permissions not granted. Please grant permissions in System Settings.",
			)
			log.Info("⚠️  Neru requires Accessibility permissions to function.")
			log.Info("Please go to: System Settings → Privacy & Security → Accessibility")
			log.Info("and enable Neru.")
			return errors.New("accessibility permissions required")
		}
	}

	// Set global config for accessibility
	config.SetGlobal(cfg)

	// Apply clickable roles if hints are enabled
	if cfg.Hints.Enabled {
		log.Info("Applying clickable roles",
			zap.Int("count", len(cfg.Hints.ClickableRoles)),
			zap.Strings("roles", cfg.Hints.ClickableRoles))
		accessibility.SetClickableRoles(cfg.Hints.ClickableRoles)
	}

	return nil
}

// initializeHotkeyService creates the hotkey service, using the provided dependency or creating a new one.
func initializeHotkeyService(deps *deps, log *zap.Logger) hotkeyService {
	if deps != nil && deps.Hotkeys != nil {
		return deps.Hotkeys
	}

	mgr := hotkeys.NewManager(log)
	hotkeys.SetGlobalManager(mgr)
	return mgr
}

// initializeAppWatcher creates and returns a new application watcher.
func initializeAppWatcher(log *zap.Logger) *appwatcher.Watcher {
	return appwatcher.NewWatcher(log)
}

// configureEventTapHotkeys configures the event tap with hotkeys from the configuration.
func (a *App) configureEventTapHotkeys(cfg *config.Config, log *zap.Logger) {
	keys := make([]string, 0, len(cfg.Hotkeys.Bindings))
	for key, value := range cfg.Hotkeys.Bindings {
		// Skip empty keys or values
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			log.Warn(
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
		if mode == domain.GetModeString(domain.ModeHints) && !cfg.Hints.Enabled {
			continue
		}
		if mode == domain.GetModeString(domain.ModeGrid) && !cfg.Grid.Enabled {
			continue
		}
		keys = append(keys, key)
	}

	// Log if no hotkeys are configured
	if len(keys) == 0 {
		log.Warn("No hotkeys configured - application will not be activatable via hotkeys")
	} else {
		log.Info("Registered hotkeys", zap.Int("count", len(keys)))
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
		a.gridComponent.Context.GetGridOverlay() != nil {
		a.overlayManager.UseGridOverlay(*a.gridComponent.Context.GetGridOverlay())
	}
}

// registerCommandHandlers registers all IPC command handlers.
func (a *App) registerCommandHandlers() {
	a.cmdHandlers[domain.CommandPing] = a.handlePing
	a.cmdHandlers[domain.CommandStart] = a.handleStart
	a.cmdHandlers[domain.CommandStop] = a.handleStop
	a.cmdHandlers[domain.GetModeString(domain.ModeHints)] = a.handleHints
	a.cmdHandlers[domain.GetModeString(domain.ModeGrid)] = a.handleGrid
	a.cmdHandlers[domain.CommandAction] = a.handleAction
	a.cmdHandlers[domain.GetModeString(domain.ModeIdle)] = a.handleIdle
	a.cmdHandlers[domain.CommandStatus] = a.handleStatus
	a.cmdHandlers[domain.CommandConfig] = a.handleConfig
	a.cmdHandlers[domain.CommandReloadConfig] = a.handleReloadConfig
	a.cmdHandlers[domain.CommandHealth] = a.handleHealth
	a.cmdHandlers[domain.CommandMetrics] = a.handleMetrics
}
