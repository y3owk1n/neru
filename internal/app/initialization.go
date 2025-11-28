package app

import (
	"context"
	"strings"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	accessibilityAdapter "github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	overlayAdapter "github.com/y3owk1n/neru/internal/core/infra/overlay"
	"github.com/y3owk1n/neru/internal/core/ports"
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
		nil,
	)
	if initConfigErr != nil {
		return nil, derrors.Wrap(initConfigErr, derrors.CodeInternal, "failed to initialize logger")
	}

	logger := logger.Get()
	bridge.InitializeLogger(logger)

	return logger, nil
}

// initializeOverlayManager creates and initializes the overlay manager.
func initializeOverlayManager(logger *zap.Logger) OverlayManager {
	return overlay.Init(logger)
}

// initializeAccessibility checks and configures accessibility permissions and settings.
func initializeAccessibility(cfg *config.Config, logger *zap.Logger) error {
	if cfg.General.AccessibilityCheckOnStart {
		if !accessibilityAdapter.CheckAccessibilityPermissions() {
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
		accessibilityAdapter.SetClickableRoles(cfg.Hints.ClickableRoles)
	}

	return nil
}

// initializeHotkeyService creates the hotkey service.
func initializeHotkeyService(logger *zap.Logger) HotkeyService {
	hotkeyManager := hotkeys.NewManager(logger)
	hotkeys.SetGlobalManager(hotkeyManager)

	return hotkeyManager
}

// initializeAppWatcher creates the app watcher.
func initializeAppWatcher(logger *zap.Logger) Watcher {
	return appwatcher.NewWatcher(logger)
}

// initializeAdapters creates and initializes the accessibility and overlay adapters.
func initializeAdapters(
	cfg *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
	metricsCollector metrics.Collector,
) (ports.AccessibilityPort, ports.OverlayPort) {
	excludedBundles := cfg.General.ExcludedApps
	clickableRoles := cfg.Hints.ClickableRoles

	// Create infrastructure client
	axClient := accessibilityAdapter.NewInfraAXClient()

	// Create base accessibility adapter with core functionality
	baseAccessibilityAdapter := accessibilityAdapter.NewAdapter(
		logger,
		excludedBundles,
		clickableRoles,
		axClient,
	)
	// Wrap with metrics decorator to track performance
	accAdapter := accessibilityAdapter.NewMetricsDecorator(
		baseAccessibilityAdapter,
		metricsCollector,
	)

	// Create overlay adapter for UI rendering
	baseOverlayAdapter := overlayAdapter.NewAdapter(overlayManager, logger)
	// Wrap with metrics decorator to track rendering performance
	overlayPort := overlayAdapter.NewMetricsDecorator(baseOverlayAdapter, metricsCollector)

	return accAdapter, overlayPort
}

// initializeServices creates and initializes the domain services.
func initializeServices(
	cfg *config.Config,
	accAdapter ports.AccessibilityPort,
	overlayAdapter ports.OverlayPort,
	logger *zap.Logger,
) (*services.HintService, *services.GridService, *services.ActionService, *services.ScrollService, error) {
	// Hint Generator - creates unique labels for UI elements
	hintGen, hintGenErr := domainHint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
	if hintGenErr != nil {
		return nil, nil, nil, nil, derrors.Wrap(
			hintGenErr,
			derrors.CodeHintGenerationFailed,
			"failed to create hint generator",
		)
	}

	// Hint Service - orchestrates hint generation and display
	hintService := services.NewHintService(accAdapter, overlayAdapter, hintGen, logger)

	// Grid Service - manages grid-based navigation overlays
	gridService := services.NewGridService(overlayAdapter, logger)

	// Action Service - handles UI element interactions
	actionService := services.NewActionService(accAdapter, overlayAdapter, cfg.Action, logger)

	// Scroll Service - manages scrolling operations
	scrollService := services.NewScrollService(accAdapter, overlayAdapter, cfg.Scroll, logger)

	return hintService, gridService, actionService, scrollService, nil
}

// processHotkeyBindings processes and filters hotkey bindings from configuration.
func processHotkeyBindings(config *config.Config, logger *zap.Logger) []string {
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

	return keys
}

// configureEventTapHotkeys configures the event tap with hotkeys from the configuration.
func (a *App) configureEventTapHotkeys(config *config.Config, logger *zap.Logger) {
	keys := processHotkeyBindings(config, logger)

	// Log if no hotkeys are configured
	if len(keys) == 0 {
		logger.Warn("No hotkeys configured - application will not be activatable via hotkeys")
	} else {
		logger.Info("Registered hotkeys", zap.Int("count", len(keys)))
	}

	a.eventTap.SetHotkeys(keys)

	// Use Background context as this is a synchronous cleanup operation
	err := a.eventTap.Disable(context.Background())
	if err != nil {
		logger.Warn("Failed to disable event tap after setting hotkeys", zap.Error(err))
	}
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
