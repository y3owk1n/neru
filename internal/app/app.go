package app

import (
	"context"
	"fmt"

	accAdapter "github.com/y3owk1n/neru/internal/adapter/accessibility"
	ovAdapter "github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/eventtap"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/overlay"
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

	state  *state.AppState
	cursor *state.CursorState

	// Core services
	overlayManager *overlay.Manager
	hotkeyManager  hotkeyService
	eventTap       eventTap
	ipcServer      ipcServer
	appWatcher     *appwatcher.Watcher

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

	// Command handlers
	cmdHandlers map[string]func(ipc.Command) ipc.Response
}

// New creates a new App instance.
func New(cfg *config.Config, configPath string) (*App, error) {
	return newWithDeps(cfg, configPath, nil)
}

func newWithDeps(cfg *config.Config, configPath string, deps *deps) (*App, error) {
	// Initialize logger
	log, err := initializeLogger(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize overlay manager
	overlayManager := initializeOverlayManager(log)

	// Initialize and check accessibility infrastructure
	err = initializeAccessibility(cfg, log)
	if err != nil {
		return nil, err
	}

	// Initialize services (Legacy)
	appWatcher := initializeAppWatcher(log)
	hotkeySvc := initializeHotkeyService(deps, log)

	// --- New Architecture Initialization ---

	// 1. Initialize Config Service
	cfgService := config.NewService(cfg, configPath)

	// 2. Initialize Adapters
	// Accessibility Adapter
	// Note: We need to get excluded bundles and clickable roles from config
	excludedBundles := cfg.General.ExcludedApps
	clickableRoles := cfg.Hints.ClickableRoles
	accAdapter := accAdapter.NewAdapter(log, excludedBundles, clickableRoles)

	// Overlay Adapter
	ovAdapter := ovAdapter.NewAdapter(overlayManager, log)

	// 3. Initialize Domain Services
	// Hint Generator
	hintGen, err := domainHint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
	if err != nil {
		return nil, fmt.Errorf("failed to create hint generator: %w", err)
	}

	// Hint Service
	hintService := services.NewHintService(accAdapter, ovAdapter, hintGen, log)

	// Grid Service
	gridService := services.NewGridService(ovAdapter, log)

	// Action Service
	actionService := services.NewActionService(accAdapter, ovAdapter, cfg.Action, log)

	// Scroll Service
	scrollService := services.NewScrollService(accAdapter, ovAdapter, cfg.Scroll, log)

	// Create app instance with basic dependencies
	app := &App{
		config:         cfg,
		ConfigPath:     configPath,
		logger:         log,
		state:          state.NewAppState(),
		cursor:         state.NewCursorState(cfg.General.RestoreCursorPosition),
		overlayManager: overlayManager,
		hotkeyManager:  hotkeySvc,
		appWatcher:     appWatcher,

		// Inject new services
		hintService:   hintService,
		gridService:   gridService,
		actionService: actionService,
		scrollService: scrollService,
		configService: cfgService,

		renderer:    &ui.OverlayRenderer{}, // Will be properly initialized later
		cmdHandlers: make(map[string]func(ipc.Command) ipc.Response),
	}

	// Initialize components using factory functions
	app.hintsComponent, err = createHintsComponent(cfg, log, overlayManager)
	if err != nil {
		return nil, err
	}

	app.gridComponent = createGridComponent(cfg, log, overlayManager)

	app.scrollComponent, err = createScrollComponent(cfg, log, overlayManager)
	if err != nil {
		return nil, err
	}

	app.actionComponent, err = createActionComponent(cfg, log, overlayManager)
	if err != nil {
		return nil, err
	}

	// Initialize renderer with component styles
	app.renderer = ui.NewOverlayRenderer(
		overlayManager,
		app.hintsComponent.Style,
		app.gridComponent.Style,
	)

	// Initialize mode handler
	app.modes = modes.NewHandler(
		cfg, log, app.state, app.cursor, overlayManager, app.renderer,
		app.hintService,
		app.gridService,
		app.actionService,
		app.scrollService,
		app.hintsComponent, app.gridComponent, app.scrollComponent, app.actionComponent,
		app.enableEventTap, app.disableEventTap,
		func() { app.refreshHotkeysForAppOrCurrent("") },
	)

	// Initialize event tap
	// Note: We pass app.HandleKeyPress which delegates to modes handler
	if deps != nil && deps.EventTapFactory != nil {
		app.eventTap = deps.EventTapFactory.New(app.HandleKeyPress, log)
	} else {
		app.eventTap = eventtap.NewEventTap(app.HandleKeyPress, log)
	}
	if app.eventTap == nil {
		log.Warn("Event tap creation failed - key capture won't work")
	} else {
		app.configureEventTapHotkeys(cfg, log)
	}

	// Initialize IPC server
	if deps != nil && deps.IPCServerFactory != nil {
		srv, srvErr := deps.IPCServerFactory.New(app.handleIPCCommand, log)
		if srvErr != nil {
			return nil, fmt.Errorf("failed to create IPC server: %w", srvErr)
		}
		app.ipcServer = srv
	} else {
		srv, srvErr := ipc.NewServer(app.handleIPCCommand, log)
		if srvErr != nil {
			return nil, fmt.Errorf("failed to create IPC server: %w", srvErr)
		}
		app.ipcServer = srv
	}

	// Register overlays with overlay manager
	app.registerOverlays()

	// Register IPC command handlers
	app.registerCommandHandlers()

	return app, nil
}

// ReloadConfig reloads the configuration from the specified path.
// If validation fails, shows an alert and keeps the current config.
// Preserves the current app state (enabled/disabled, current mode).
func (a *App) ReloadConfig(configPath string) error {
	// Load new config with validation
	result := config.LoadWithValidation(configPath)

	// If there's a validation error, show alert and keep current config
	if result.ValidationError != nil {
		a.logger.Warn("Config validation failed during reload",
			zap.Error(result.ValidationError),
			zap.String("config_path", result.ConfigPath))

		// Show alert dialog
		bridge.ShowConfigValidationError(result.ValidationError.Error(), result.ConfigPath)

		return fmt.Errorf("config validation failed: %w", result.ValidationError)
	}

	// Exit current mode before updating config
	if a.state.CurrentMode() != ModeIdle {
		a.ExitMode()
	}

	// Unregister all current hotkeys before updating config
	if a.state.HotkeysRegistered() {
		a.logger.Info("Unregistering current hotkeys before reload")
		a.hotkeyManager.UnregisterAll()
		a.state.SetHotkeysRegistered(false)
	}

	// Update config
	a.config = result.Config
	a.ConfigPath = result.ConfigPath

	// Update global config for accessibility package
	config.SetGlobal(result.Config)

	// Update accessibility roles if hints config changed
	if result.Config.Hints.Enabled {
		a.logger.Info("Updating clickable roles",
			zap.Int("count", len(result.Config.Hints.ClickableRoles)))
		infra.SetClickableRoles(result.Config.Hints.ClickableRoles)
	}

	// Reconfigure event tap hotkeys with new config
	a.configureEventTapHotkeys(result.Config, a.logger)

	// Update all components with new config
	a.hintsComponent.UpdateConfig(result.Config, a.logger)
	a.gridComponent.UpdateConfig(result.Config, a.logger)
	a.scrollComponent.UpdateConfig(result.Config, a.logger)
	a.actionComponent.UpdateConfig(result.Config, a.logger)

	// Update modes handler with new config
	if a.modes != nil {
		a.modes.UpdateConfig(result.Config)
	}

	// Re-register global hotkeys with new config
	a.refreshHotkeysForAppOrCurrent("")

	a.logger.Info("Configuration reloaded successfully")
	return nil
}

// ActivateMode activates the specified mode.
func (a *App) ActivateMode(mode Mode) { a.modes.ActivateMode(mode) }

// SetEnabled sets the enabled state of the application.
func (a *App) SetEnabled(v bool) { a.state.SetEnabled(v) }

// IsEnabled returns the enabled state of the application.
func (a *App) IsEnabled() bool { return a.state.IsEnabled() }

// HintsEnabled returns true if hints are enabled.
func (a *App) HintsEnabled() bool { return a.config != nil && a.config.Hints.Enabled }

// GridEnabled returns true if grid is enabled.
func (a *App) GridEnabled() bool { return a.config != nil && a.config.Grid.Enabled }

// Config returns the application configuration.
func (a *App) Config() *config.Config { return a.config }

// Logger returns the application logger.
func (a *App) Logger() *zap.Logger { return a.logger }

// OverlayManager returns the overlay manager.
func (a *App) OverlayManager() *overlay.Manager { return a.overlayManager }

// HintsContext returns the hints context.
func (a *App) HintsContext() *hints.Context { return a.hintsComponent.Context }

// Renderer returns the overlay renderer.
func (a *App) Renderer() *ui.OverlayRenderer { return a.renderer }

// GetConfigPath returns the config path.
func (a *App) GetConfigPath() string { return a.ConfigPath }

// SetHintOverlayNeedsRefresh sets the hint overlay needs refresh flag.
func (a *App) SetHintOverlayNeedsRefresh(value bool) { a.state.SetHintOverlayNeedsRefresh(value) }

// CaptureInitialCursorPosition captures the initial cursor position.
func (a *App) CaptureInitialCursorPosition() { a.modes.CaptureInitialCursorPosition() }

// IsFocusedAppExcluded checks if the focused app is excluded.
func (a *App) IsFocusedAppExcluded() bool {
	// Use ActionService to check exclusion
	ctx := context.Background()
	excluded, err := a.actionService.IsFocusedAppExcluded(ctx)
	if err != nil {
		a.logger.Warn("Failed to check exclusion", zap.Error(err))
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
func (a *App) EventTap() eventTap { return a.eventTap }

// CurrentMode returns the current mode.
func (a *App) CurrentMode() Mode { return a.state.CurrentMode() }

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
