package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	infra "github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	eventtapadapter "github.com/y3owk1n/neru/internal/core/infra/eventtap"
	ipcadapter "github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
	"go.uber.org/zap"
)

// Option is a functional option for configuring an App instance.
type Option func(*App) error

// WithConfig sets the application configuration.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) error {
		a.config = cfg

		return nil
	}
}

// WithConfigPath sets the configuration file path.
func WithConfigPath(path string) Option {
	return func(a *App) error {
		a.ConfigPath = path

		return nil
	}
}

// WithLogger sets the application logger.
func WithLogger(logger *zap.Logger) Option {
	return func(a *App) error {
		a.logger = logger

		return nil
	}
}

// WithEventTap sets the event tap implementation.
func WithEventTap(eventTap ports.EventTapPort) Option {
	return func(a *App) error {
		a.eventTap = eventTap

		return nil
	}
}

// WithIPCServer sets the IPC server implementation.
func WithIPCServer(ipcServer ports.IPCPort) Option {
	return func(a *App) error {
		a.ipcServer = ipcServer

		return nil
	}
}

// WithOverlayManager sets the overlay manager implementation.
func WithOverlayManager(overlayManager OverlayManager) Option {
	return func(a *App) error {
		a.overlayManager = overlayManager

		return nil
	}
}

// WithWatcher sets the app watcher implementation.
func WithWatcher(watcher Watcher) Option {
	return func(a *App) error {
		a.appWatcher = watcher

		return nil
	}
}

// WithHotkeyService sets the hotkey service implementation.
func WithHotkeyService(hotkeyService HotkeyService) Option {
	return func(a *App) error {
		a.hotkeyManager = hotkeyService

		return nil
	}
}

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
	eventTap       ports.EventTapPort
	ipcServer      ports.IPCPort
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

	// Lifecycle management
	gcCancel context.CancelFunc

	// Renderer
	renderer *ui.OverlayRenderer

	// IPC Controller
	ipcController *IPCController
}

// New creates a new App instance with the provided options.
// It applies sensible defaults and allows customization through functional options.
func New(opts ...Option) (*App, error) {
	app := &App{}

	// Apply all options
	for _, opt := range opts {
		err := opt(app)
		if err != nil {
			return nil, err
		}
	}

	// Set defaults for required fields if not provided
	if app.config == nil {
		app.config = config.DefaultConfig()
	}

	if app.logger == nil {
		logger, err := initializeLogger(app.config)
		if err != nil {
			return nil, err
		}

		app.logger = logger
	}

	// Initialize the rest of the application
	return initializeApp(app)
}

// initializeApp completes the initialization of an App instance that has been
// partially configured with options. It sets up all the remaining dependencies,
// services, and components.
func initializeApp(app *App) (*App, error) {
	cfg := app.config
	logger := app.logger

	// Initialize overlay manager if not provided
	if app.overlayManager == nil {
		app.overlayManager = initializeOverlayManager(logger)
	}

	// Initialize accessibility infrastructure early as it's required by adapters
	accessibilityErr := initializeAccessibility(cfg, logger)
	if accessibilityErr != nil {
		return nil, accessibilityErr
	}

	// Initialize app watcher if not provided
	if app.appWatcher == nil {
		app.appWatcher = initializeAppWatcher(logger)
	}

	// Initialize hotkey service if not provided
	if app.hotkeyManager == nil {
		app.hotkeyManager = initializeHotkeyService(logger)
	}

	// Initialize config service
	cfgService := config.NewService(cfg, app.ConfigPath)

	// Initialize metrics
	var metricsCollector metrics.Collector
	if cfg.Metrics.Enabled {
		metricsCollector = metrics.NewCollector()
	} else {
		metricsCollector = &metrics.NoOpCollector{}
	}

	app.metrics = metricsCollector

	// Initialize adapters
	accAdapter, overlayAdapter := initializeAdapters(
		cfg,
		logger,
		app.overlayManager,
		metricsCollector,
	)

	// Initialize services
	hintService, gridService, actionService, scrollService, servicesErr := initializeServices(
		cfg,
		accAdapter,
		overlayAdapter,
		logger,
	)
	if servicesErr != nil {
		return nil, servicesErr
	}

	// Set remaining app fields
	app.appState = state.NewAppState()
	app.cursorState = state.NewCursorState(cfg.General.RestoreCursorPosition)
	app.hintService = hintService
	app.gridService = gridService
	app.actionService = actionService
	app.scrollService = scrollService
	app.configService = cfgService
	app.renderer = &ui.OverlayRenderer{} // Will be properly initialized later

	// Create UI components for different interaction modes
	hintsComponent, hintsComponentErr := createHintsComponent(cfg, logger, app.overlayManager)
	if hintsComponentErr != nil {
		return nil, hintsComponentErr
	}

	app.hintsComponent = hintsComponent
	app.gridComponent = createGridComponent(cfg, logger, app.overlayManager)

	scrollComponent, scrollComponentErr := createScrollComponent(cfg, logger, app.overlayManager)
	if scrollComponentErr != nil {
		return nil, scrollComponentErr
	}

	app.scrollComponent = scrollComponent

	actionComponent, actionComponentErr := createActionComponent(cfg, logger, app.overlayManager)
	if actionComponentErr != nil {
		return nil, actionComponentErr
	}

	app.actionComponent = actionComponent

	app.renderer = ui.NewOverlayRenderer(
		app.overlayManager,
		app.hintsComponent.Style,
		app.gridComponent.Style,
	)

	// Register overlays with overlay manager
	app.registerOverlays()

	// Initialize mode handler that coordinates different interaction modes
	app.modes = modes.NewHandler(
		cfg, logger, app.appState, app.cursorState, app.overlayManager, app.renderer,
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
		app.ConfigPath,
	)

	// Initialize event tap if not provided
	if app.eventTap == nil {
		tap := eventtapadapter.NewEventTap(app.HandleKeyPress, logger)
		app.eventTap = eventtapadapter.NewAdapter(tap, logger)
	}

	if app.eventTap == nil {
		logger.Warn("Event tap creation failed - key capture won't work")
	} else {
		app.configureEventTapHotkeys(cfg, logger)
	}

	// Initialize IPC server if not provided
	if app.ipcServer == nil {
		server, serverErr := ipcadapter.NewServer(app.ipcController.HandleCommand, logger)
		if serverErr != nil {
			return nil, derrors.Wrap(
				serverErr,
				derrors.CodeIPCFailed,
				"failed to create IPC server",
			)
		}

		app.ipcServer = ipcadapter.NewAdapter(server, logger)
	}

	return app, nil
}

// ReloadConfig reloads the configuration from the specified path.
// If validation fails, shows an alert and keeps the current config.
// Preserves the current app state (enabled/disabled, current mode).
func (a *App) ReloadConfig(configPath string) error {
	configResult, err := a.validateConfigReload(configPath)
	if err != nil {
		return err
	}

	a.prepareForConfigUpdate()
	a.applyConfigUpdate(configResult)
	a.reconfigureAfterUpdate(configResult)

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
func (a *App) EventTap() ports.EventTapPort { return a.eventTap }

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

// validateConfigReload loads and validates the config, handling validation errors.
func (a *App) validateConfigReload(configPath string) (*config.LoadResult, error) {
	configResult := config.LoadWithValidation(configPath)

	if configResult.ValidationError != nil {
		a.logger.Warn("Config validation failed during reload",
			zap.Error(configResult.ValidationError),
			zap.String("config_path", configResult.ConfigPath))

		bridge.ShowConfigValidationError(
			configResult.ValidationError.Error(),
			configResult.ConfigPath,
		)

		return configResult, derrors.Wrap(
			configResult.ValidationError,
			derrors.CodeInvalidConfig,
			"config validation failed",
		)
	}

	return configResult, nil
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

// applyConfigUpdate applies the new config to the app state.
func (a *App) applyConfigUpdate(configResult *config.LoadResult) {
	a.config = configResult.Config
	a.ConfigPath = configResult.ConfigPath

	config.SetGlobal(configResult.Config)

	if configResult.Config.Hints.Enabled {
		a.logger.Info("Updating clickable roles",
			zap.Int("count", len(configResult.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(configResult.Config.Hints.ClickableRoles, a.logger)
	}
}

// reconfigureAfterUpdate reconfigures components and services after config update.
func (a *App) reconfigureAfterUpdate(configResult *config.LoadResult) {
	a.configureEventTapHotkeys(configResult.Config, a.logger)

	a.hintsComponent.UpdateConfig(configResult.Config, a.logger)
	a.gridComponent.UpdateConfig(configResult.Config, a.logger)
	a.scrollComponent.UpdateConfig(configResult.Config, a.logger)
	a.actionComponent.UpdateConfig(configResult.Config, a.logger)

	if a.modes != nil {
		a.modes.UpdateConfig(configResult.Config)
	}

	a.refreshHotkeysForAppOrCurrent("")
}

// Helper methods for event tap control (used by callbacks)

func (a *App) enableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Enable(context.Background())
		if err != nil {
			a.logger.Error("Failed to enable event tap", zap.Error(err))
		}
	}
}

func (a *App) disableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Disable(context.Background())
		if err != nil {
			a.logger.Error("Failed to disable event tap", zap.Error(err))
		}
	}
}
