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
	// Phase 1: Initialize core infrastructure
	err := initializeInfrastructure(app)
	if err != nil {
		return nil, err
	}

	// Phase 2: Initialize services and adapters
	err = initializeServicesAndAdapters(app)
	if err != nil {
		return nil, err
	}

	// Phase 3: Initialize application state
	initializeApplicationState(app)

	// Phase 4: Initialize UI components
	err = initializeUIComponents(app)
	if err != nil {
		return nil, err
	}

	// Phase 5: Initialize renderer and register overlays
	initializeRendererAndOverlays(app)

	// Phase 6: Initialize mode handler
	initializeModeHandler(app)

	// Phase 7: Initialize IPC controller
	initializeIPCController(app)

	// Phase 8: Initialize event tap and IPC server
	err = initializeEventTapAndIPC(app)
	if err != nil {
		return nil, err
	}

	// Phase 9: Initialize shutdown channel
	initializeShutdownChannel(app)

	return app, nil
}
