package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	eventtapadapter "github.com/y3owk1n/neru/internal/core/infra/eventtap"
	ipcadapter "github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/ui"
	"go.uber.org/zap"
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
	// Get styles with nil-safe fallbacks
	var hintStyle hints.StyleMode
	if app.hintsComponent != nil {
		hintStyle = app.hintsComponent.Style
	} else {
		// Fallback to default style if component is nil
		hintStyle = hints.BuildStyle(config.DefaultConfig().Hints)
	}

	var gridStyle grid.Style
	if app.gridComponent != nil {
		gridStyle = app.gridComponent.Style
	} else {
		// Fallback to default style if component is nil
		gridStyle = grid.BuildStyle(config.DefaultConfig().Grid)
	}

	app.renderer = ui.NewOverlayRenderer(
		app.overlayManager,
		hintStyle,
		gridStyle,
	)

	// Register overlays with overlay manager
	app.registerOverlays()
}

// initializeModeHandler creates and configures the mode handler that
// coordinates different interaction modes.
func initializeModeHandler(app *App) {
	cfg := app.config
	logger := app.logger

	// Group related dependencies for better readability
	deps := struct {
		config         *config.Config
		logger         *zap.Logger
		appState       *state.AppState
		cursorState    *state.CursorState
		overlayManager OverlayManager
		renderer       *ui.OverlayRenderer
		services       struct {
			hint   *services.HintService
			grid   *services.GridService
			action *services.ActionService
			scroll *services.ScrollService
		}
		components struct {
			hints  *components.HintsComponent
			grid   *components.GridComponent
			scroll *components.ScrollComponent
			action *components.ActionComponent
		}
		callbacks struct {
			enableEventTap  func()
			disableEventTap func()
			refreshHotkeys  func()
		}
	}{
		config:         cfg,
		logger:         logger,
		appState:       app.appState,
		cursorState:    app.cursorState,
		overlayManager: app.overlayManager,
		renderer:       app.renderer,
		services: struct {
			hint   *services.HintService
			grid   *services.GridService
			action *services.ActionService
			scroll *services.ScrollService
		}{
			hint:   app.hintService,
			grid:   app.gridService,
			action: app.actionService,
			scroll: app.scrollService,
		},
		components: struct {
			hints  *components.HintsComponent
			grid   *components.GridComponent
			scroll *components.ScrollComponent
			action *components.ActionComponent
		}{
			hints:  app.hintsComponent,
			grid:   app.gridComponent,
			scroll: app.scrollComponent,
			action: app.actionComponent,
		},
		callbacks: struct {
			enableEventTap  func()
			disableEventTap func()
			refreshHotkeys  func()
		}{
			enableEventTap:  app.enableEventTap,
			disableEventTap: app.disableEventTap,
			refreshHotkeys:  func() { app.refreshHotkeysForAppOrCurrent("") },
		},
	}

	app.modes = modes.NewHandler(
		deps.config,
		deps.logger,
		deps.appState,
		deps.cursorState,
		deps.overlayManager,
		deps.renderer,
		deps.services.hint,
		deps.services.grid,
		deps.services.action,
		deps.services.scroll,
		deps.components.hints,
		deps.components.grid,
		deps.components.scroll,
		deps.components.action,
		deps.callbacks.enableEventTap,
		deps.callbacks.disableEventTap,
		deps.callbacks.refreshHotkeys,
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
	}

	if app.eventTap != nil {
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

// cleanupInfrastructure cleans up resources allocated during infrastructure initialization.
func cleanupInfrastructure(app *App) {
	// Clean up hotkey service
	if app.hotkeyManager != nil {
		app.hotkeyManager.UnregisterAll()
		app.hotkeyManager = nil
	}

	// Clean up app watcher
	if app.appWatcher != nil {
		app.appWatcher.Stop()
		app.appWatcher = nil
	}

	// Note: overlayManager and accessibility don't need explicit cleanup here
	// as they're handled by the main Cleanup() method
}

// cleanupServicesAndAdapters cleans up resources allocated during services initialization.
func cleanupServicesAndAdapters(app *App) {
	// Services are cleaned up by their respective Close methods when the app is properly initialized
	// For partial cleanup, we just nil out the references
	app.hintService = nil
	app.gridService = nil
	app.actionService = nil
	app.scrollService = nil
	app.configService = nil
	app.metrics = nil
}

// cleanupUIComponents cleans up resources allocated during UI components initialization.
func cleanupUIComponents(app *App) {
	// UI components are cleaned up by the overlay manager when overlays are destroyed
	// For partial cleanup, we just nil out the references
	app.hintsComponent = nil
	app.gridComponent = nil
	app.scrollComponent = nil
	app.actionComponent = nil

	// Clean up renderer
	app.renderer = nil
}

// cleanupEventTapAndIPC cleans up resources allocated during event tap and IPC initialization.
func cleanupEventTapAndIPC(app *App) {
	// Clean up IPC server
	if app.ipcServer != nil {
		// Try to stop the server gracefully
		stopErr := app.ipcServer.Stop(context.Background())
		if stopErr != nil {
			app.logger.Error("Failed to stop IPC server during cleanup", zap.Error(stopErr))
		}

		app.ipcServer = nil
	}

	// Clean up event tap
	if app.eventTap != nil {
		app.eventTap.Destroy()
		app.eventTap = nil
	}

	// Clean up IPC controller
	app.ipcController = nil
}
