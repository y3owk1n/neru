package app

import (
	"strings"

	"go.uber.org/zap"

	accessibilityAdapter "github.com/y3owk1n/neru/internal/adapter/accessibility"
	accessibilityNative "github.com/y3owk1n/neru/internal/adapter/accessibility/native"
	"github.com/y3owk1n/neru/internal/adapter/appwatcher"
	"github.com/y3owk1n/neru/internal/adapter/hotkeys"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/adapter/overlay"
	visionAdapter "github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/app/keybinding"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/app/services/virtualpointer"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
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
		roles := cfg.Hints.ResolvedClickableRoles()

		logger.Debug("Applying clickable roles",
			zap.Int("configured", len(cfg.Hints.ClickableRoles)),
			zap.Int("resolved", len(roles)))
		accessibilityNative.SetClickableRoles(roles, logger)
	}

	return nil
}

// initializeHotkeyService creates the hotkey service.
func initializeHotkeyService(logger *zap.Logger) HotkeyService {
	return hotkeys.NewManager(logger)
}

// initializeAppWatcher creates the app watcher.
func initializeAppWatcher(logger *zap.Logger) Watcher {
	return appwatcher.NewWatcher(logger)
}

// initializeAdapters creates and initializes the accessibility and overlay adapters.
func initializeAdapters(
	app *App,
	cfg *config.Config,
	cfgService *loader.Service,
	logger *zap.Logger,
	overlayManager OverlayManager,
	systemPort ports.SystemPort,
) (ports.AccessibilityPort, ports.OverlayPort) {
	// Respect an injected accessibility port (WithAccessibility); only build
	// the real adapter and its platform AX client when none was provided.
	accAdapter := app.accessibility
	if accAdapter == nil {
		excludedBundles := cfg.General.ExcludedApps
		clickableRoles := cfg.Hints.ClickableRoles

		// Create infrastructure client.
		axClient := accessibilityAdapter.NewPlatformAXClient(logger, cfgService)

		// Store axClient so Cleanup can release its resources (D-Bus conn, a11y status).
		app.axClient = axClient

		// Create base accessibility adapter with core functionality
		accAdapter = accessibilityAdapter.NewAdapter(
			logger,
			excludedBundles,
			clickableRoles,
			axClient,
			cfg.Hints.DetectMissionControl,
		)
	}

	// The overlay owns config + theme -> Style. Building it here, before any
	// render component exists, means every later consumer reads the same
	// resolved values rather than deriving its own.
	app.overlayStyles = overlay.NewStyleResolver(
		overlayManager,
		cfg,
		newThemeProvider(systemPort),
		logger,
	)

	// Create overlay adapter for UI rendering
	overlayPort := overlay.NewAdapter(
		overlayManager,
		app.overlayStyles,
		logger,
	)

	return accAdapter, overlayPort
}

// indicatorServices are the three services that each own one indicator end to
// end: whether it is on screen, how big its surface is, and what it draws.
// They are built the same way from the same ports, so they travel together.
type indicatorServices struct {
	mode           *modeindicator.Service
	sticky         *stickyindicator.Service
	virtualPointer *virtualpointer.Service
}

// newIndicatorServices builds the service for each indicator.
func newIndicatorServices(
	overlayAdapter ports.OverlayPort,
	systemPort ports.SystemPort,
) indicatorServices {
	return indicatorServices{
		mode:           modeindicator.NewService(systemPort, overlayAdapter),
		sticky:         stickyindicator.NewService(systemPort, overlayAdapter),
		virtualPointer: virtualpointer.NewService(systemPort, overlayAdapter),
	}
}

// initializeServices creates and initializes the domain services.
func initializeServices(
	cfg *config.Config,
	accAdapter ports.AccessibilityPort,
	overlayAdapter ports.OverlayPort,
	systemPort ports.SystemPort,
	logger *zap.Logger,
) (*services.HintService, *services.GridService, *services.ActionService, *services.ScrollService, indicatorServices, error) {
	// Hint Generator - creates unique labels for UI elements
	hintGen, hintGenErr := domainHint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		domainHint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if hintGenErr != nil {
		return nil, nil, nil, nil, indicatorServices{}, derrors.Wrap(
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

	// Grid Service - reports overlay health for grid mode
	gridService := services.NewGridService(overlayAdapter)

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
		systemPort,
		cfg.Scroll,
		logger,
	)

	// Indicator services - each owns one indicator, visibility and position
	indicators := newIndicatorServices(overlayAdapter, systemPort)

	return hintService, gridService, actionService, scrollService, indicators, nil
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

		if keybinding.ActionsReferenceDisabledMode(actions, cfg) {
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
