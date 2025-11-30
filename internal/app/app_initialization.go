package app

import (
	"github.com/y3owk1n/neru/internal/config"
)

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
// partially configured with options. It orchestrates the initialization of all
// application components in the correct order.
func initializeApp(app *App) (*App, error) {
	var initializedPhases []func() // Cleanup functions for successful phases

	var initializationFailed bool

	// Cleanup function that runs on failure to prevent resource leaks
	defer func() {
		if initializationFailed {
			app.logger.Info("Initialization failed, cleaning up partially allocated resources")
			// Run cleanup functions in reverse order (LIFO)
			for i := len(initializedPhases) - 1; i >= 0; i-- {
				initializedPhases[i]()
			}
		}
	}()

	// Phase 1: Initialize core infrastructure
	err := initializeInfrastructure(app)
	if err != nil {
		initializationFailed = true

		return nil, err
	}

	initializedPhases = append(initializedPhases, func() {
		cleanupInfrastructure(app)
	})

	// Phase 2: Initialize services and adapters
	err = initializeServicesAndAdapters(app)
	if err != nil {
		initializationFailed = true

		return nil, err
	}

	initializedPhases = append(initializedPhases, func() {
		cleanupServicesAndAdapters(app)
	})

	// Phase 3: Initialize application state
	initializeApplicationState(app)
	// Application state doesn't need cleanup as it's just in-memory objects

	// Phase 4: Initialize UI components
	err = initializeUIComponents(app)
	if err != nil {
		initializationFailed = true

		return nil, err
	}

	initializedPhases = append(initializedPhases, func() {
		cleanupUIComponents(app)
	})

	// Phase 5: Initialize renderer and register overlays
	initializeRendererAndOverlays(app)
	// Renderer and overlays are cleaned up as part of UI components

	// Phase 6: Initialize mode handler
	initializeModeHandler(app)
	// Mode handler cleanup is handled by the mode handler itself

	// Phase 7: Initialize IPC controller
	initializeIPCController(app)
	// IPC controller doesn't need specific cleanup beyond what services provide

	// Phase 8: Initialize event tap and IPC server
	err = initializeEventTapAndIPC(app)
	if err != nil {
		initializationFailed = true

		return nil, err
	}

	initializedPhases = append(initializedPhases, func() {
		cleanupEventTapAndIPC(app)
	})

	// Phase 9: Initialize shutdown channel
	initializeShutdownChannel(app)
	// Shutdown channel doesn't need cleanup

	return app, nil
}
