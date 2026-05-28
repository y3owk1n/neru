package app

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/config"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	accessibilityAdapter "github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	overlayAdapter "github.com/y3owk1n/neru/internal/core/infra/overlay"
	visionAdapter "github.com/y3owk1n/neru/internal/core/infra/vision"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// initializeLogger initializes the application logger with the given configuration.
func initializeLogger(cfg *config.Config) (*zap.Logger, error) {
	initConfigErr := logger.Init(
		cfg.Logging.LogLevel,
		cfg.Logging.LogFile,
		cfg.Logging.DisableFileLogging,
		cfg.Logging.MaxFileSize,
		cfg.Logging.MaxBackups,
		cfg.Logging.MaxAge,
		nil,
	)
	if initConfigErr != nil {
		return nil, derrors.Wrap(initConfigErr, derrors.CodeInternal, "failed to initialize logger")
	}

	logger := logger.Get().Named("app")
	initializePlatformLogger(logger)

	return logger, nil
}

// initializeOverlayManager creates and initializes the overlay manager.
func initializeOverlayManager(logger *zap.Logger) OverlayManager {
	return overlay.Init(logger)
}

// initializeAccessibility checks and configures accessibility permissions and settings.
func initializeAccessibility(cfg *config.Config, logger *zap.Logger) error {
	// Apply clickable roles if hints are enabled
	if cfg.Hints.Enabled {
		logger.Debug("Applying clickable roles",
			zap.Int("count", len(cfg.Hints.ClickableRoles)))
		accessibilityAdapter.SetClickableRoles(cfg.Hints.ClickableRoles, logger)
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
	cfgService *config.Service,
	logger *zap.Logger,
	overlayManager OverlayManager,
	systemPort ports.SystemPort,
) (ports.AccessibilityPort, ports.OverlayPort) {
	excludedBundles := cfg.General.ExcludedApps
	clickableRoles := cfg.Hints.ClickableRoles

	// Create infrastructure client
	axClient := accessibilityAdapter.NewInfraAXClient(logger, cfgService)

	// Create base accessibility adapter with core functionality
	accAdapter := accessibilityAdapter.NewAdapter(
		logger,
		excludedBundles,
		clickableRoles,
		axClient,
		cfg.Hints.DetectMissionControl,
	)

	// Create overlay adapter for UI rendering
	overlayPort := overlayAdapter.NewAdapter(
		overlayManager,
		newThemeProvider(systemPort),
		systemPort,
		logger,
	)

	return accAdapter, overlayPort
}

// initializeServices creates and initializes the domain services.
func initializeServices(
	cfg *config.Config,
	accAdapter ports.AccessibilityPort,
	overlayAdapter ports.OverlayPort,
	systemPort ports.SystemPort,
	logger *zap.Logger,
) (*services.HintService, *services.GridService, *services.ActionService, *services.ScrollService, *modeindicator.Service, *stickyindicator.Service, error) {
	// Hint Generator - creates unique labels for UI elements
	hintGen, hintGenErr := domainHint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
	if hintGenErr != nil {
		return nil, nil, nil, nil, nil, nil, derrors.Wrap(
			hintGenErr,
			derrors.CodeHintGenerationFailed,
			"failed to create hint generator",
		)
	}

	// Vision adapter - vision-based element detection (optional, used on "vision" strategy)
	visionPort := visionAdapter.NewAdapter(logger)

	// Hint Service - orchestrates hint generation and display
	hintService := services.NewHintService(
		accAdapter,
		overlayAdapter,
		systemPort,
		hintGen,
		cfg.Hints,
		logger,
		visionPort,
	)

	// Grid Service - manages grid-based navigation overlays
	gridService := services.NewGridService(overlayAdapter, systemPort, logger)

	// Action Service - handles UI element interactions
	actionService := services.NewActionService(
		accAdapter,
		overlayAdapter,
		systemPort,
		logger,
	)
	actionService.UpdateConfig(cfg.MouseAction)

	// Scroll Service - manages scrolling operations
	scrollService := services.NewScrollService(
		accAdapter,
		overlayAdapter,
		systemPort,
		cfg.Scroll,
		logger,
	)

	// Mode Indicator Service - manages mode indicator overlay
	modeIndicatorService := modeindicator.NewService(
		systemPort,
		overlayAdapter,
		logger,
	)

	// Sticky Indicator Service - manages sticky modifiers indicator overlay
	stickyIndicatorService := stickyindicator.NewService(
		systemPort,
		overlayAdapter,
		logger,
	)

	return hintService, gridService, actionService, scrollService, modeIndicatorService, stickyIndicatorService, nil
}

// processHotkeyBindings processes and filters hotkey bindings from configuration.
func processHotkeyBindings(cfg *config.Config, logger *zap.Logger) []string {
	keys := make([]string, 0, len(cfg.Hotkeys.Bindings))
	for key, actions := range cfg.Hotkeys.Bindings {
		// Skip empty keys or empty action arrays
		if strings.TrimSpace(key) == "" || len(actions) == 0 {
			logger.Warn(
				"Skipping empty hotkey binding",
				zap.String("key", key),
				zap.Int("action_count", len(actions)),
			)

			continue
		}

		if actionsReferenceDisabledMode(actions, cfg) {
			continue
		}

		// Canonicalize the key to convert "Primary" to platform-specific modifier ("Cmd" on macOS)
		canonicalKey := config.CanonicalHotkeyForPlatform(key)
		keys = append(keys, canonicalKey)
	}

	return keys
}

// configureEventTapHotkeys configures the event tap with hotkeys from the configuration.
func (a *App) configureEventTapHotkeys(cfg *config.Config, logger *zap.Logger) {
	layoutID := strings.TrimSpace(cfg.General.KBLayoutToUse)

	layoutResolved := a.eventTap.SetKeyboardLayout(layoutID)
	if layoutID != "" && !layoutResolved {
		logger.Warn("Configured keyboard layout was not found; using automatic fallback",
			zap.String("layout_id", layoutID))
	}

	keys := processHotkeyBindings(cfg, logger)

	// Log hotkey registration status
	if len(keys) == 0 {
		logger.Info(
			"No hotkeys configured — use CLI commands (neru hints, neru grid, etc.) to trigger modes",
		)
	} else {
		logger.Info("Registered hotkeys", zap.Int("count", len(keys)))
	}

	a.eventTap.SetHotkeys(keys)

	err := a.eventTap.Disable(a.ctx)
	if err != nil {
		logger.Warn("Failed to disable event tap after setting hotkeys", zap.Error(err))
	}
}

// registerOverlays registers all component overlays with the overlay manager.
func (a *App) registerOverlays() {
	if a.modeIndicatorComponent != nil && a.modeIndicatorComponent.Overlay != nil {
		a.overlayManager.UseModeIndicatorOverlay(a.modeIndicatorComponent.Overlay)
	}

	if a.stickyIndicatorComponent != nil && a.stickyIndicatorComponent.Overlay != nil {
		a.overlayManager.UseStickyModifiersOverlay(a.stickyIndicatorComponent.Overlay)
	}

	if a.hintsComponent != nil && a.hintsComponent.Overlay != nil {
		a.overlayManager.UseHintOverlay(a.hintsComponent.Overlay)
	}

	if a.gridComponent != nil && a.gridComponent.Overlay != nil {
		a.overlayManager.UseGridOverlay(a.gridComponent.Overlay)
	}

	if a.recursiveGridComponent != nil && a.recursiveGridComponent.Overlay != nil {
		a.overlayManager.UseRecursiveGridOverlay(a.recursiveGridComponent.Overlay)
	}
}
