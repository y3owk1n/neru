package app

import (
	"context"

	"go.uber.org/zap"

	eventtapadapter "github.com/y3owk1n/neru/internal/adapter/eventtap"
	ipcadapter "github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/adapter/keyfeed"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	infrasystray "github.com/y3owk1n/neru/internal/adapter/systray"
	textinputadapter "github.com/y3owk1n/neru/internal/adapter/textinput"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/systray"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/keybinding"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
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

	// Initialize the overlay backend unless a port was injected in its place.
	// A caller that brought its own ports.OverlayPort brought its own screen
	// with it, and building a native backend behind it would put a second one
	// there.
	if app.overlayPort == nil {
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
	cfgService := app.newConfigService(logger)
	configurePlatformRuntimeConfigProviders(cfgService, logger)

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
	app.overlayPort = overlayAdapter

	hintService, gridService, actionService, scrollService, indicators, err := initializeServices(
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
	app.indicators = indicators
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

// initializeUIComponents asks the overlay to build the components it draws
// through, then assembles the per-mode components around them.
//
// The overlay is handed the configuration and a theme provider and nothing
// else: the surface those components attach to is the overlay's own, and
// nothing built there comes back — since #1213 no app-layer component holds a
// render object. Most components that fail to build are logged and left nil;
// the virtual pointer is the one whose failure still fails this phase, as it
// always has. A session running on an injected port has no backend to ask, and
// nothing to build against.
func initializeUIComponents(app *App) error {
	if app.overlayManager != nil {
		_, err := app.overlayManager.BuildComponents(
			app.config,
			newThemeProvider(app.systemPort),
		)
		if err != nil {
			// The overlay may have built components before it failed, and some
			// of them own native windows. This phase's cleanup closure is only
			// registered once the phase succeeds, so releasing them is this
			// call's job rather than the unwind's.
			app.overlayManager.Destroy()

			return err
		}
	}

	factory := NewComponentFactory(
		app.config,
		app.logger,
		app.overlayPort,
	)

	app.hintsComponent = factory.CreateHintsComponent()
	app.gridComponent = factory.CreateGridComponent()
	app.scrollComponent = factory.CreateScrollComponent()
	app.recursiveGridComponent = factory.CreateRecursiveGridComponent()

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

// configureRenderComponents gives the render components their first
// configuration, by resolving the Style once now that they exist.
func configureRenderComponents(app *App) {
	// The render components exist now, so the overlay can hand them their
	// configuration the same way a later reload or theme change does. Without
	// this they would only ever be configured by their constructors, and the
	// first draw would take a different path from every draw after it.
	if app.overlayPort != nil {
		app.overlayPort.RefreshStyles()
	}
}

// initializeModeHandler creates and configures the mode handler that
// coordinates different interaction modes.
func initializeModeHandler(app *App) {
	cfg := app.config
	logger := app.logger

	// Group related dependencies for better readability
	deps := struct {
		config      *config.Config
		logger      *zap.Logger
		appState    *state.AppState
		cursorState *state.CursorState
		overlayPort ports.OverlayPort
		services    struct {
			hint       *services.HintService
			grid       *services.GridService
			action     *services.ActionService
			scroll     *services.ScrollService
			indicators indicatorServices
		}
		components struct {
			hints         *components.HintsComponent
			grid          *components.GridComponent
			scroll        *components.ScrollComponent
			recursivegrid *components.RecursiveGridComponent
		}
		callbacks struct {
			refreshHotkeys        func()
			executeActionSequence func(source string, steps []string)
		}
	}{
		config:      cfg,
		logger:      logger,
		appState:    app.appState,
		cursorState: app.cursorState,
		overlayPort: app.overlayPort,
		services: struct {
			hint       *services.HintService
			grid       *services.GridService
			action     *services.ActionService
			scroll     *services.ScrollService
			indicators indicatorServices
		}{
			hint:       app.hintService,
			grid:       app.gridService,
			action:     app.actionService,
			scroll:     app.scrollService,
			indicators: app.indicators,
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
			refreshHotkeys        func()
			executeActionSequence func(source string, steps []string)
		}{
			refreshHotkeys:        func() { app.hotkeys.RefreshFor("") },
			executeActionSequence: app.runActionSequence,
		},
	}

	app.modes = modes.NewHandler(modes.HandlerDeps{
		Ctx:                    app.ctx,
		Config:                 deps.config,
		Logger:                 deps.logger,
		AppState:               deps.appState,
		CursorState:            deps.cursorState,
		OverlayPort:            deps.overlayPort,
		HintService:            deps.services.hint,
		GridService:            deps.services.grid,
		ActionService:          deps.services.action,
		ScrollService:          deps.services.scroll,
		ModeIndicatorService:   deps.services.indicators.mode,
		StickyIndicatorService: deps.services.indicators.sticky,
		VirtualPointerService:  deps.services.indicators.virtualPointer,
		HintsComponent:         deps.components.hints,
		GridComponent:          deps.components.grid,
		ScrollComponent:        deps.components.scroll,
		RecursiveGridComponent: deps.components.recursivegrid,
		RefreshHotkeys:         deps.callbacks.refreshHotkeys,
		ExecuteActionSequence:  deps.callbacks.executeActionSequence,
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
	app.ipcController = ipcctrl.New(ipcctrl.Deps{
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
		KeyFeed:         app.keyFeed,
		ReloadConfig:    app.ReloadConfig,
		ExecuteSequence: app.executeActionSequenceWithPolicy,
		ExecuteMacro:    app.executeMacro,
		Logger:          app.logger,
	})

	// Build the sequence executor now that the controller it dispatches
	// through exists. Until this point the App falls back to constructing one
	// per call, which is correct but pays for it on every key press.
	app.sequenceExecutor = app.newSequenceExecutor()

	// The binder needs the executor, so it is built here rather than with the
	// rest of the infrastructure.
	app.hotkeys = keybinding.New(hotkeyBinderDeps(app))

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
		if app.overlayPort != nil {
			app.overlayPort.SetHiddenInScreenShare(hidden)
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

// hotkeyBinderDeps is what the hotkey binder is built from.
//
// It is a function rather than a literal at the call site so the one thing about
// it that is easy to get wrong can be tested: this phase runs before the phase
// that builds the event tap, so every dependency here has to tolerate a field
// that is still nil. Reading one *now* to hand over a method value panics on the
// spot, which is a crash before the daemon has logged anything it could be
// diagnosed from (TestHotkeyBinderDeps_PublishSurvivesAnEventTapThatDoesNotExistYet).
func hotkeyBinderDeps(app *App) keybinding.Deps {
	return keybinding.Deps{
		Manager:     app.hotkeyManager,
		Modes:       app.modes,
		State:       app.appState,
		FocusedApp:  app.actionService,
		Config:      app.configSnapshot,
		RunSequence: app.runActionSequence,
		// The taps hand a registered chord back to the mechanism that owns it, so
		// they are told what registration actually took rather than what the
		// configuration asked for. Late-bound for the reason above: the field is
		// filled by phase 8 and the nil check keeps a startup that never reaches
		// it harmless.
		PublishRegisteredHotkeys: func(keys []string) {
			if app.eventTap != nil {
				app.eventTap.SetHotkeys(keys)
			}
		},
		Context: func() context.Context { return app.ctx },
		Logger:  app.logger,
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

			// A backend that presents an action's modifiers by pressing real
			// keys has to say so, or the tap reads its own injection as the
			// user pressing that modifier. Which backends need it is a
			// per-platform answer; the concrete tap is only in reach here.
			registerSyntheticModifierSink(tap, logger)
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
		app.hotkeys.Unregister()
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
	app.indicators = indicatorServices{}
	app.configService = nil
}

// cleanupUIComponents cleans up resources allocated during UI components initialization.
func cleanupUIComponents(app *App) {
	// UI components are cleaned up by the overlay manager when overlays are destroyed
	// For partial cleanup, we just nil out the references
	app.hintsComponent = nil
	app.gridComponent = nil
	app.scrollComponent = nil
	app.recursiveGridComponent = nil
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

		// The injection backend holds the tap in a slot of its own, which
		// outlives this App unless it is let go of here.
		registerSyntheticModifierSink(nil, app.logger)
	}

	// Clean up IPC controller
	app.ipcController = nil
}
