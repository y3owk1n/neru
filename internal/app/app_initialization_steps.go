package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/systray"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/config"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	eventtapadapter "github.com/y3owk1n/neru/internal/core/infra/eventtap"
	ipcadapter "github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/keyfeed"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	infrasystray "github.com/y3owk1n/neru/internal/core/infra/systray"
	textinputadapter "github.com/y3owk1n/neru/internal/core/infra/textinput"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
)

// initializeInfrastructure sets up the core infrastructure components
// that are needed by other parts of the application.
func initializeInfrastructure(app *App) error {
	cfg := app.config
	logger := app.logger

	// Initialize platform system port if not provided
	if app.systemPort == nil {
		systemPort, err := platform.NewSystemPort()
		if err != nil {
			return err
		}

		app.systemPort = systemPort
	}

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

	if app.textInput == nil {
		app.textInput = textinputadapter.NewAdapter(textinputadapter.NewTextInput(logger), logger)
	}

	if app.keyFeed == nil {
		app.keyFeed = keyfeed.NewAdapter(logger)
	}

	// Install the platform font resolver. SetFontResolver makes the
	// resolver available globally to every call site that builds an
	// overlay style, so they do not have to thread the port through
	// their own signatures.
	if resolver := platform.NewFontResolver(); resolver != nil {
		ports.SetFontResolver(resolver)
	}

	return nil
}

// initializeServicesAndAdapters sets up all the service layer components
// and their required adapters.
func initializeServicesAndAdapters(app *App) error {
	cfg := app.config
	logger := app.logger

	// Initialize config service
	cfgService := config.NewService(cfg, app.ConfigPath, logger, app.systemPort)
	configurePlatformRuntimeConfigProviders(cfgService)

	// Initialize adapters
	// accAdapter is retained on the App because the focus-change path calls
	// PrimeApplication directly, outside any service.
	accAdapter, overlayAdapter := initializeAdapters(
		app,
		cfg,
		cfgService,
		logger,
		app.overlayManager,
		app.systemPort,
	)
	app.accessibility = accAdapter

	// Initialize services
	hintService, gridService, actionService, scrollService, modeIndicatorService, stickyIndicatorService, err := initializeServices(
		cfg,
		accAdapter,
		overlayAdapter,
		app.systemPort,
		logger,
	)
	if err != nil {
		return err
	}

	// Pre-build a generator for the opposite label direction so the
	// per-activation `hints --label-direction <opposite>` path resolves to
	// a real generator instead of silently falling back to the default.
	registerOppositeLabelDirectionGenerator(app, hintService, cfg)

	// Store services on app
	app.hintService = hintService
	app.gridService = gridService
	app.actionService = actionService
	app.scrollService = scrollService
	app.modeIndicatorService = modeIndicatorService
	app.stickyIndicatorService = stickyIndicatorService
	app.configService = cfgService

	return nil
}

// registerOppositeLabelDirectionGenerator builds and registers a generator for
// the label direction opposite to the configured one, so the per-activation
// `hints --label-direction <opposite>` override resolves to a real generator
// instead of silently falling back to the default.
func registerOppositeLabelDirectionGenerator(
	app *App,
	hintService *services.HintService,
	cfg *config.Config,
) {
	if hintService == nil {
		return
	}

	primaryDirection := domainHint.LabelDirectionFromString(
		cfg.Hints.LabelDirectionForApp(""),
	)

	oppositeDirection := primaryDirection.Opposite()

	if primaryDirection == oppositeDirection {
		return
	}

	oppositeGen, oppositeGenErr := domainHint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		oppositeDirection,
	)
	if oppositeGenErr != nil {
		app.logger.Error(
			"Failed to build opposite-direction hint generator",
			zap.String("direction", oppositeDirection.String()),
			zap.Error(oppositeGenErr),
		)

		return
	}

	hintService.UpdateGenerator(app.ctx, oppositeGen)

	app.logger.Debug(
		"Registered opposite-direction hint generator",
		zap.String("primary_direction", primaryDirection.String()),
		zap.String("opposite_direction", oppositeDirection.String()),
	)
}

// initializeApplicationState sets up the core application state objects.
func initializeApplicationState(app *App) {
	app.appState = state.NewAppState()
	app.cursorState = state.NewCursorState()
}

// initializeUIComponents creates and configures all UI components
// for the different interaction modes.
func initializeUIComponents(app *App) error {
	factory := NewComponentFactory(app.config, app.logger, app.overlayManager, app.systemPort)

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
		OverlayType:    "",
	})
	if err != nil {
		return err
	}

	app.scrollComponent = scrollComponent

	modeIndicatorComponent, err := factory.CreateModeIndicatorComponent(ComponentCreationOptions{
		SkipIfDisabled: false,
		Required:       false,
		OverlayType:    "mode_indicator",
	})
	if err != nil {
		return err
	}

	app.modeIndicatorComponent = modeIndicatorComponent

	stickyIndicatorComponent, err := factory.CreateStickyIndicatorComponent(
		ComponentCreationOptions{
			SkipIfDisabled: false,
			Required:       false,
			OverlayType:    "sticky_modifiers",
		},
	)
	if err != nil {
		return err
	}

	app.stickyIndicatorComponent = stickyIndicatorComponent

	virtualPointerOverlay, err := factory.CreateVirtualPointerOverlay()
	if err != nil {
		return err
	}

	app.virtualPointerOverlay = virtualPointerOverlay

	recursiveGridComponent, err := factory.CreateRecursiveGridComponent(ComponentCreationOptions{
		SkipIfDisabled: false,
		Required:       false,
		OverlayType:    "recursive_grid",
	})
	if err != nil {
		return err
	}

	app.recursiveGridComponent = recursiveGridComponent

	return nil
}

// initializeSystrayComponent creates and configures the systray component.
func initializeSystrayComponent(app *App) {
	systrayComponent := systray.NewComponent(
		app,
		infrasystray.NewAdapter(),
		app.systemPort,
		app.logger,
	)
	app.systrayComponent = systrayComponent
}

// initializeRendererAndOverlays sets up the overlay renderer and registers
// all overlays with the overlay manager.
func initializeRendererAndOverlays(app *App) {
	// Build styles directly from config + live theme provider rather than
	// reading component fields that would go stale after config reloads.
	cfg := config.DefaultConfig()

	if app.config != nil {
		cfg = app.config
	}

	hintStyle := hints.BuildStyle(cfg.Hints, app.systemPort)
	gridStyle := grid.BuildStyle(cfg.Grid, app.systemPort)
	recursiveGridStyle := recursivegrid.BuildStyle(cfg.RecursiveGrid, app.systemPort)

	app.renderer = ui.NewOverlayRenderer(
		app.overlayManager,
		hintStyle,
		gridStyle,
		recursiveGridStyle,
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
			hint            *services.HintService
			grid            *services.GridService
			action          *services.ActionService
			scroll          *services.ScrollService
			modeIndicator   *modeindicator.Service
			stickyIndicator *stickyindicator.Service
		}
		components struct {
			hints         *components.HintsComponent
			grid          *components.GridComponent
			scroll        *components.ScrollComponent
			recursivegrid *components.RecursiveGridComponent
		}
		callbacks struct {
			refreshHotkeys      func()
			executeHotkeyAction func(key, actionStr string) error
		}
	}{
		config:         cfg,
		logger:         logger,
		appState:       app.appState,
		cursorState:    app.cursorState,
		overlayManager: app.overlayManager,
		renderer:       app.renderer,
		services: struct {
			hint            *services.HintService
			grid            *services.GridService
			action          *services.ActionService
			scroll          *services.ScrollService
			modeIndicator   *modeindicator.Service
			stickyIndicator *stickyindicator.Service
		}{
			hint:            app.hintService,
			grid:            app.gridService,
			action:          app.actionService,
			scroll:          app.scrollService,
			modeIndicator:   app.modeIndicatorService,
			stickyIndicator: app.stickyIndicatorService,
		},
		components: struct {
			hints         *components.HintsComponent
			grid          *components.GridComponent
			scroll        *components.ScrollComponent
			recursivegrid *components.RecursiveGridComponent
		}{
			hints:         app.hintsComponent,
			grid:          app.gridComponent,
			scroll:        app.scrollComponent,
			recursivegrid: app.recursiveGridComponent,
		},
		callbacks: struct {
			refreshHotkeys      func()
			executeHotkeyAction func(key, actionStr string) error
		}{
			refreshHotkeys:      func() { app.refreshHotkeysForAppOrCurrent("") },
			executeHotkeyAction: app.executeHotkeyAction,
		},
	}

	app.modes = modes.NewHandler(modes.HandlerDeps{
		Ctx:                    app.ctx,
		Config:                 deps.config,
		Logger:                 deps.logger,
		AppState:               deps.appState,
		CursorState:            deps.cursorState,
		OverlayManager:         deps.overlayManager,
		Renderer:               deps.renderer,
		HintService:            deps.services.hint,
		GridService:            deps.services.grid,
		ActionService:          deps.services.action,
		ScrollService:          deps.services.scroll,
		ModeIndicatorService:   deps.services.modeIndicator,
		StickyIndicatorService: deps.services.stickyIndicator,
		HintsComponent:         deps.components.hints,
		GridComponent:          deps.components.grid,
		ScrollComponent:        deps.components.scroll,
		RecursiveGridComponent: deps.components.recursivegrid,
		RefreshHotkeys:         deps.callbacks.refreshHotkeys,
		ExecuteHotkeyAction:    deps.callbacks.executeHotkeyAction,
		Shutdown:               app.Quit,
		TextInput:              app.textInput,
		System:                 app.systemPort,
	})
}

// initializeIPCController sets up the IPC controller for external communication.
// Note: eventTap and ipcServer are nil at this point; they are set later
// via SetInfrastructure in initializeEventTapAndIPC (Phase 8). The
// SetConfigField callback is set after creation so the constructor's
// signature stays stable for test callers.
func initializeIPCController(app *App) {
	app.ipcController = NewIPCController(IPCControllerDeps{
		HintService:   app.hintService,
		GridService:   app.gridService,
		ActionService: app.actionService,
		ScrollService: app.scrollService,
		ConfigService: app.configService,
		AppState:      app.appState,
		Config:        app.config,
		Modes:         app.modes,
		System:        app.systemPort,
		// EventTap and IPCServer stay zero here; phase 8 fills them in
		// through SetInfrastructure.
		KeyFeed:      app.keyFeed,
		ReloadConfig: app.ReloadConfig,
		Logger:       app.logger,
	})

	// Set the config-set callback so runtime field changes propagate to
	// app components (services, overlays, hotkeys, etc.). Uses the setter
	// method so it also propagates to the info handler (which was created
	// during construction before the callback was available).
	app.ipcController.SetConfigFieldCallback(app.SetConfigField)
}

// setupScreenShareStateSubscription sets up a callback to update overlay when screen share state changes.
func setupScreenShareStateSubscription(app *App) {
	// Apply initial config value to overlay if set to hide in screen share.
	// The OnScreenShareStateChanged callback fires immediately with the current
	// state, so no direct SetSharingType call is needed here.
	if app.config != nil && app.config.General.HideOverlayInScreenShare {
		app.appState.SetHiddenForScreenShare(true)
	}

	app.screenShareSubscriptionID = app.appState.OnScreenShareStateChanged(func(hidden bool) {
		if app.overlayManager != nil {
			app.overlayManager.SetSharingType(hidden)
		}
	})
}

// cleanupScreenShareStateSubscription cleans up the screen share state subscription.
func cleanupScreenShareStateSubscription(app *App) {
	if app.screenShareSubscriptionID != 0 {
		app.appState.OffScreenShareStateChanged(app.screenShareSubscriptionID)
		app.screenShareSubscriptionID = 0
	}
}

// initializeEventTapAndIPC sets up the event tap for key capture and
// the IPC server for external communication.
func initializeEventTapAndIPC(app *App) error {
	cfg := app.config
	logger := app.logger

	// Initialize event tap if not provided
	if app.eventTap == nil {
		tap := eventtapadapter.NewEventTap(app.HandleKeyPress, logger)
		if tap != nil {
			app.eventTap = eventtapadapter.NewAdapter(tap, logger)
		}
	}

	if app.eventTap == nil {
		logger.Warn("Event tap creation failed - key capture won't work")
	}

	if app.eventTap != nil {
		app.configureEventTapHotkeys(cfg, logger)
	}

	// Register Go-level keyboard layout change handler so CGEventTap hotkeys
	// (registered with raw keycodes) are re-registered when the layout changes.
	app.registerLayoutChangeHandler()

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

	// Update the IPC controller with the now-initialized infrastructure
	// references so the health handler can query their state.
	if app.ipcController != nil {
		app.ipcController.SetInfrastructure(app.eventTap, app.ipcServer)
	}

	// The mode handler is built in phase 7, before the event tap exists, so it
	// receives the port here rather than through its constructor.
	if app.modes != nil {
		app.modes.SetEventTap(app.eventTap)
	}

	return nil
}

// initializeShutdownChannel creates the stop channel for programmatic shutdown.
func initializeShutdownChannel(app *App) {
	app.stopChan = make(chan struct{})
}

// cleanupInfrastructure cleans up resources allocated during infrastructure initialization.
func cleanupInfrastructure(app *App) {
	// Unregister layout change handler so a stale callback cannot fire
	// after the App is torn down.
	app.unregisterLayoutChangeHandler()

	// Clean up hotkey service
	if app.hotkeyManager != nil {
		app.hotkeyRegistrationMu.Lock()
		app.stopAllHotkeyRepeats()
		app.hotkeyManager.UnregisterAll()
		app.appState.SetHotkeysRegistered(false)
		app.hotkeyRegistrationMu.Unlock()
		app.hotkeyManager = nil
	}

	// Clean up app watcher
	if app.appWatcher != nil {
		app.appWatcher.Stop()
		app.appWatcher = nil
	}

	// Note: overlayManager doesn't need explicit cleanup here as it's handled
	// by the main Cleanup() method.
}

// cleanupServicesAndAdapters cleans up resources allocated during services initialization.
func cleanupServicesAndAdapters(app *App) {
	// Services are cleaned up by their respective Close methods when the app is properly initialized
	// For partial cleanup, we just nil out the references
	app.hintService = nil
	app.gridService = nil
	app.actionService = nil
	app.scrollService = nil
	app.modeIndicatorService = nil
	app.stickyIndicatorService = nil
	app.configService = nil
}

// cleanupUIComponents cleans up resources allocated during UI components initialization.
func cleanupUIComponents(app *App) {
	// UI components are cleaned up by the overlay manager when overlays are destroyed
	// For partial cleanup, we just nil out the references
	app.hintsComponent = nil
	app.gridComponent = nil
	app.scrollComponent = nil
	app.modeIndicatorComponent = nil
	app.stickyIndicatorComponent = nil
	app.virtualPointerOverlay = nil

	// Clean up renderer
	app.renderer = nil
}

// cleanupEventTapAndIPC cleans up resources allocated during event tap and IPC initialization.
func cleanupEventTapAndIPC(app *App) {
	// Clean up IPC server
	if app.ipcServer != nil {
		// Try to stop the server gracefully.
		// Use a fresh context since app.ctx may already be canceled.
		stopCtx, stopCancel := context.WithTimeout(context.Background(), StopTimeout)

		stopErr := app.ipcServer.Stop(stopCtx)

		stopCancel()

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
