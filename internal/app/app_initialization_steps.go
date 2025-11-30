package app

import (
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	eventtapadapter "github.com/y3owk1n/neru/internal/core/infra/eventtap"
	ipcadapter "github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/ui"
)

// initializeInfrastructure sets up the core infrastructure components
// that are needed by other parts of the application.
func initializeInfrastructure(app *App) error {
	cfg := app.config
	logger := app.logger

	// Initialize overlay manager if not provided
	if app.overlayManager == nil {
		app.overlayManager = initializeOverlayManager(logger)
	}

	// Initialize accessibility infrastructure early as it's required by adapters
	err := initializeAccessibility(cfg, logger)
	if err != nil {
		return err
	}

	// Initialize app watcher if not provided
	if app.appWatcher == nil {
		app.appWatcher = initializeAppWatcher(logger)
	}

	// Initialize hotkey service if not provided
	if app.hotkeyManager == nil {
		app.hotkeyManager = initializeHotkeyService(logger)
	}

	return nil
}

// initializeServicesAndAdapters sets up all the service layer components
// and their required adapters.
func initializeServicesAndAdapters(app *App) error {
	cfg := app.config
	logger := app.logger

	// Initialize config service
	cfgService := config.NewService(cfg, app.ConfigPath, logger)

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
	hintService, gridService, actionService, scrollService, err := initializeServices(
		cfg,
		accAdapter,
		overlayAdapter,
		logger,
	)
	if err != nil {
		return err
	}

	// Store services on app
	app.hintService = hintService
	app.gridService = gridService
	app.actionService = actionService
	app.scrollService = scrollService
	app.configService = cfgService

	return nil
}

// initializeApplicationState sets up the core application state objects.
func initializeApplicationState(app *App) {
	cfg := app.config

	app.appState = state.NewAppState()
	app.cursorState = state.NewCursorState(cfg.General.RestoreCursorPosition)
}

// initializeUIComponents creates and configures all UI components
// for the different interaction modes.
func initializeUIComponents(app *App) error {
	factory := NewComponentFactory(app.config, app.logger, app.overlayManager)

	// Create UI components for different interaction modes with standardized patterns
	hintsComponent, err := factory.CreateHintsComponent(ComponentCreationOptions{
		SkipIfDisabled: true,
		Required:       false,
		OverlayType:    "hints",
	})
	if err != nil {
		return err
	}

	app.hintsComponent = hintsComponent

	gridComponent, err := factory.CreateGridComponent(ComponentCreationOptions{
		SkipIfDisabled: false, // Grid needs minimal context even when disabled
		Required:       false,
		OverlayType:    "grid",
	})
	if err != nil {
		return err
	}

	app.gridComponent = gridComponent

	scrollComponent, err := factory.CreateScrollComponent(ComponentCreationOptions{
		SkipIfDisabled: false,
		Required:       false,
		OverlayType:    "scroll",
	})
	if err != nil {
		return err
	}

	app.scrollComponent = scrollComponent

	actionComponent, err := factory.CreateActionComponent(ComponentCreationOptions{
		SkipIfDisabled: false,
		Required:       false,
		OverlayType:    "action",
	})
	if err != nil {
		return err
	}

	app.actionComponent = actionComponent

	return nil
}

// initializeRendererAndOverlays sets up the overlay renderer and registers
// all overlays with the overlay manager.
func initializeRendererAndOverlays(app *App) {
	app.renderer = ui.NewOverlayRenderer(
		app.overlayManager,
		app.hintsComponent.Style,
		app.gridComponent.Style,
	)

	// Register overlays with overlay manager
	app.registerOverlays()
}

// initializeModeHandler creates and configures the mode handler that
// coordinates different interaction modes.
func initializeModeHandler(app *App) {
	cfg := app.config
	logger := app.logger

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
}

// initializeIPCController sets up the IPC controller for external communication.
func initializeIPCController(app *App) {
	app.ipcController = NewIPCController(
		app.hintService,
		app.gridService,
		app.actionService,
		app.scrollService,
		app.configService,
		app.appState,
		app.config,
		app.modes,
		app.logger,
		app.metrics,
		app.ConfigPath,
	)
}

// initializeEventTapAndIPC sets up the event tap for key capture and
// the IPC server for external communication.
func initializeEventTapAndIPC(app *App) error {
	cfg := app.config
	logger := app.logger

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
		server, err := ipcadapter.NewServer(app.ipcController.HandleCommand, logger)
		if err != nil {
			return derrors.Wrap(
				err,
				derrors.CodeIPCFailed,
				"failed to create IPC server",
			)
		}

		app.ipcServer = ipcadapter.NewAdapter(server, logger)
	}

	return nil
}

// initializeShutdownChannel creates the stop channel for programmatic shutdown.
func initializeShutdownChannel(app *App) {
	app.stopChan = make(chan struct{})
}
