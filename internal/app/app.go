package app

import (
	"context"

	accessibilityAdapter "github.com/y3owk1n/neru/internal/adapter/accessibility"
	overlayAdapter "github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/eventtap"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/metrics"
	"github.com/y3owk1n/neru/internal/ui"
	"go.uber.org/zap"
)

// Mode is the current mode of the application.
type Mode = domain.Mode

// Mode constants from domain package.
const (
	ModeIdle  = domain.ModeIdle
	ModeHints = domain.ModeHints
	ModeGrid  = domain.ModeGrid
)

// App represents the main application instance containing all state and dependencies.
type App struct {
	config     *config.Config
	ConfigPath string
	logger     *zap.Logger

	appState    *state.AppState
	cursorState *state.CursorState

	// Core services
	overlayManager OverlayManager
	hotkeyManager  HotkeyService
	eventTap       EventTap
	ipcServer      IPCServer
	appWatcher     Watcher
	metrics        metrics.Collector

	modes *modes.Handler

	// New Architecture Services
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	configService *config.Service

	// Feature components
	hintsComponent  *components.HintsComponent
	gridComponent   *components.GridComponent
	scrollComponent *components.ScrollComponent
	actionComponent *components.ActionComponent

	// Renderer
	renderer *ui.OverlayRenderer

	// IPC Controller
	ipcController *IPCController
}

// New creates a new application instance with default dependencies.
func New(config *config.Config, configPath string) (*App, error) {
	return newWithDeps(config, configPath, nil)
}

// NewWithDeps creates a new application instance with injected dependencies.
// This is primarily used for testing.
func NewWithDeps(config *config.Config, configPath string, deps *Deps) (*App, error) {
	return newWithDeps(config, configPath, deps)
}

// newWithDeps initializes the application with all dependencies, services, and components.
// It orchestrates the creation of adapters, services, and UI components in the correct order,
// ensuring proper dependency injection and error handling throughout the initialization process.
func newWithDeps(cfg *config.Config, configPath string, deps *Deps) (*App, error) {
	logger, loggerErr := initializeLogger(cfg)
	if loggerErr != nil {
		return nil, loggerErr
	}

	overlayManager := initializeOverlayManager(deps, logger)

	// Initialize accessibility infrastructure early as it's required by adapters
	accessibilityErr := initializeAccessibility(cfg, logger)
	if accessibilityErr != nil {
		return nil, accessibilityErr
	}

	appWatcher := initializeAppWatcher(deps, logger)
	hotkeySvc := initializeHotkeyService(deps, logger)

	cfgService := config.NewService(cfg, configPath)

	var metricsCollector metrics.Collector
	if cfg.Metrics.Enabled {
		metricsCollector = metrics.NewCollector()
	} else {
		metricsCollector = &metrics.NoOpCollector{}
	}

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
	overlayAdapter := overlayAdapter.NewMetricsDecorator(baseOverlayAdapter, metricsCollector)

	// Initialize domain services that encapsulate business logic
	// Hint Generator - creates unique labels for UI elements
	hintGen, hintGenErr := domainHint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
	if hintGenErr != nil {
		return nil, derrors.Wrap(
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

	// Create app instance with basic dependencies
	app := &App{
		config:         cfg,
		ConfigPath:     configPath,
		logger:         logger,
		appState:       state.NewAppState(),
		cursorState:    state.NewCursorState(cfg.General.RestoreCursorPosition),
		overlayManager: overlayManager,
		hotkeyManager:  hotkeySvc,
		appWatcher:     appWatcher,
		metrics:        metricsCollector,

		// Inject new services
		hintService:   hintService,
		gridService:   gridService,
		actionService: actionService,
		scrollService: scrollService,
		configService: cfgService,

		renderer: &ui.OverlayRenderer{}, // Will be properly initialized later
	}

	// Create UI components for different interaction modes
	hintsComponent, hintsComponentErr := createHintsComponent(cfg, logger, overlayManager)
	if hintsComponentErr != nil {
		return nil, hintsComponentErr
	}

	app.hintsComponent = hintsComponent

	app.gridComponent = createGridComponent(cfg, logger, overlayManager)

	scrollComponent, scrollComponentErr := createScrollComponent(cfg, logger, overlayManager)
	if scrollComponentErr != nil {
		return nil, scrollComponentErr
	}

	app.scrollComponent = scrollComponent

	actionComponent, actionComponentErr := createActionComponent(cfg, logger, overlayManager)
	if actionComponentErr != nil {
		return nil, actionComponentErr
	}

	app.actionComponent = actionComponent

	app.renderer = ui.NewOverlayRenderer(
		overlayManager,
		app.hintsComponent.Style,
		app.gridComponent.Style,
	)

	// Initialize mode handler that coordinates different interaction modes
	app.modes = modes.NewHandler(
		cfg, logger, app.appState, app.cursorState, overlayManager, app.renderer,
		app.hintService,
		app.gridService,
		app.actionService,
		app.scrollService,
		app.hintsComponent, app.gridComponent, app.scrollComponent, app.actionComponent,
		app.enableEventTap, app.disableEventTap,
		func() { app.refreshHotkeysForAppOrCurrent("") },
	)

	// Set up IPC controller for external communication
	app.ipcController = NewIPCController(
		hintService,
		gridService,
		actionService,
		scrollService,
		cfgService,
		app.appState,
		app.config,
		app.modes,
		logger,
		metricsCollector,
		configPath,
	)

	// Note: We pass app.HandleKeyPress which delegates to modes handler
	if deps != nil && deps.EventTapFactory != nil {
		app.eventTap = deps.EventTapFactory.New(app.HandleKeyPress, logger)
	} else {
		app.eventTap = eventtap.NewEventTap(app.HandleKeyPress, logger)
	}

	if app.eventTap == nil {
		logger.Warn("Event tap creation failed - key capture won't work")
	} else {
		app.configureEventTapHotkeys(cfg, logger)
	}

	if deps != nil && deps.IPCServerFactory != nil {
		server, serverErr := deps.IPCServerFactory.New(app.ipcController.HandleCommand, logger)
		if serverErr != nil {
			return nil, derrors.Wrap(
				serverErr,
				derrors.CodeIPCFailed,
				"failed to create IPC server",
			)
		}

		app.ipcServer = server
	} else {
		server, serverErr := ipc.NewServer(app.ipcController.HandleCommand, logger)
		if serverErr != nil {
			return nil, derrors.Wrap(serverErr, derrors.CodeIPCFailed, "failed to create IPC server")
		}

		app.ipcServer = server
	}

	// Register overlays with overlay manager
	app.registerOverlays()

	return app, nil
}

// ReloadConfig reloads the configuration from the specified path.
// If validation fails, shows an alert and keeps the current config.
// Preserves the current app state (enabled/disabled, current mode).
func (a *App) ReloadConfig(configPath string) error {
	// Load new config with validation
	configResult := config.LoadWithValidation(configPath)

	// If there's a validation error, show alert and keep current config
	if configResult.ValidationError != nil {
		a.logger.Warn("Config validation failed during reload",
			zap.Error(configResult.ValidationError),
			zap.String("config_path", configResult.ConfigPath))

		// Show alert dialog
		bridge.ShowConfigValidationError(
			configResult.ValidationError.Error(),
			configResult.ConfigPath,
		)

		return derrors.Wrap(
			configResult.ValidationError,
			derrors.CodeInvalidConfig,
			"config validation failed",
		)
	}

	// Exit current mode before updating config
	if a.appState.CurrentMode() != ModeIdle {
		a.ExitMode()
	}

	// Unregister all current hotkeys before updating config
	if a.appState.HotkeysRegistered() {
		a.logger.Info("Unregistering current hotkeys before reload")
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)
	}

	// Update config
	a.config = configResult.Config
	a.ConfigPath = configResult.ConfigPath

	// Update global config for accessibility package
	config.SetGlobal(configResult.Config)

	// Update accessibility roles if hints config changed
	if configResult.Config.Hints.Enabled {
		a.logger.Info("Updating clickable roles",
			zap.Int("count", len(configResult.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(configResult.Config.Hints.ClickableRoles)
	}

	// Reconfigure event tap hotkeys with new config
	a.configureEventTapHotkeys(configResult.Config, a.logger)

	// Update all components with new config
	a.hintsComponent.UpdateConfig(configResult.Config, a.logger)
	a.gridComponent.UpdateConfig(configResult.Config, a.logger)
	a.scrollComponent.UpdateConfig(configResult.Config, a.logger)
	a.actionComponent.UpdateConfig(configResult.Config, a.logger)

	// Update modes handler with new config
	if a.modes != nil {
		a.modes.UpdateConfig(configResult.Config)
	}

	// Re-register global hotkeys with new config
	a.refreshHotkeysForAppOrCurrent("")

	a.logger.Info("Configuration reloaded successfully")

	return nil
}

// ActivateMode activates the specified mode.
func (a *App) ActivateMode(mode Mode) {
	a.modes.ActivateMode(mode)
}

// SetEnabled sets the enabled state of the application.
func (a *App) SetEnabled(v bool) {
	a.appState.SetEnabled(v)
}

// IsEnabled returns the enabled state of the application.
func (a *App) IsEnabled() bool {
	return a.appState.IsEnabled()
}

// HintsEnabled returns true if hints are enabled.
func (a *App) HintsEnabled() bool {
	return a.config != nil && a.config.Hints.Enabled
}

// GridEnabled returns true if grid is enabled.
func (a *App) GridEnabled() bool {
	return a.config != nil && a.config.Grid.Enabled
}

// Config returns the application configuration.
func (a *App) Config() *config.Config {
	return a.config
}

// Logger returns the application logger.
func (a *App) Logger() *zap.Logger {
	return a.logger
}

// OverlayManager returns the overlay manager.
func (a *App) OverlayManager() OverlayManager {
	return a.overlayManager
}

// HintsContext returns the hints context.
func (a *App) HintsContext() *hints.Context {
	return a.hintsComponent.Context
}

// Renderer returns the overlay renderer.
func (a *App) Renderer() *ui.OverlayRenderer {
	return a.renderer
}

// GetConfigPath returns the config path.
func (a *App) GetConfigPath() string {
	return a.ConfigPath
}

// SetHintOverlayNeedsRefresh sets the hint overlay needs refresh flag.
func (a *App) SetHintOverlayNeedsRefresh(
	value bool,
) {
	a.appState.SetHintOverlayNeedsRefresh(value)
}

// CaptureInitialCursorPosition captures the initial cursor position.
func (a *App) CaptureInitialCursorPosition() { a.modes.CaptureInitialCursorPosition() }

// IsFocusedAppExcluded checks if the focused app is excluded.
func (a *App) IsFocusedAppExcluded() bool {
	// Use ActionService to check exclusion
	ctx := context.Background()

	excluded, excludedErr := a.actionService.IsFocusedAppExcluded(ctx)
	if excludedErr != nil {
		a.logger.Warn("Failed to check exclusion", zap.Error(excludedErr))

		return false
	}

	return excluded
}

// ExitMode exits the current mode.
func (a *App) ExitMode() { a.modes.ExitMode() }

// GridContext returns the grid context.
func (a *App) GridContext() *grid.Context { return a.gridComponent.Context }

// ScrollContext returns the scroll context.
func (a *App) ScrollContext() *scroll.Context { return a.scrollComponent.Context }

// EventTap returns the event tap.
func (a *App) EventTap() EventTap { return a.eventTap }

// CurrentMode returns the current mode.
func (a *App) CurrentMode() Mode { return a.appState.CurrentMode() }

// SetModeHints sets the mode to hints.
func (a *App) SetModeHints() { a.modes.SetModeHints() }

// SetModeGrid sets the mode to grid.
func (a *App) SetModeGrid() { a.modes.SetModeGrid() }

// SetModeIdle sets the mode to idle.
func (a *App) SetModeIdle() { a.modes.SetModeIdle() }

// EnableEventTap enables the event tap.
func (a *App) EnableEventTap() { a.enableEventTap() }

// DisableEventTap disables the event tap.
func (a *App) DisableEventTap() { a.disableEventTap() }

// HandleKeyPress delegates key press handling to the mode handler.
func (a *App) HandleKeyPress(key string) {
	a.modes.HandleKeyPress(key)
}

// Helper methods for event tap control (used by callbacks)

func (a *App) enableEventTap() {
	if a.eventTap != nil {
		a.eventTap.Enable()
	}
}

func (a *App) disableEventTap() {
	if a.eventTap != nil {
		a.eventTap.Disable()
	}
}
