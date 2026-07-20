package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// IPCController handles IPC command routing and execution.
type IPCController struct {
	// Services
	HintService   *services.HintService
	GridService   *services.GridService
	ActionService *services.ActionService
	ScrollService *services.ScrollService
	ConfigService *config.Service

	// State
	AppState *state.AppState

	// Infrastructure
	Logger    *zap.Logger
	System    ports.SystemPort
	EventTap  ports.EventTapPort
	IPCServer ports.IPCPort

	// Mode management
	Modes *modes.Handler

	// Reload callback for full app-level config reload
	ReloadConfig func(ctx context.Context, configPath string) error

	// SetConfigField callback for runtime config field changes with full
	// app-level reconfiguration (component updates, hotkey re-registration, etc.).
	// If nil, the config is only updated in-memory.
	SetConfigField func(ctx context.Context, key, value string) error

	// Info handler for config updates
	infoHandler *IPCControllerInfo

	// Command Handlers map
	Handlers map[string]func(context.Context, ipc.Command) ipc.Response
}

// NewIPCController creates a new IPC controller with the given dependencies.
func NewIPCController(
	hintService *services.HintService,
	gridService *services.GridService,
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	configService *config.Service,
	appState *state.AppState,
	config *config.Config,
	modesHandler *modes.Handler,
	systemPort ports.SystemPort,
	eventTap ports.EventTapPort,
	ipcServer ports.IPCPort,
	reloadConfig func(ctx context.Context, configPath string) error,
	logger *zap.Logger,
) *IPCController {
	if logger == nil {
		logger = zap.NewNop()
	}

	ipcController := &IPCController{
		HintService:   hintService,
		GridService:   gridService,
		ActionService: actionService,
		ScrollService: scrollService,
		ConfigService: configService,
		AppState:      appState,
		Modes:         modesHandler,
		System:        systemPort,
		EventTap:      eventTap,
		IPCServer:     ipcServer,
		ReloadConfig:  reloadConfig,
		Logger:        logger.Named("ipc.controller"),
		Handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}

	// Register command handlers
	ipcController.registerHandlers(config)

	return ipcController
}

// HandleCommand routes an IPC command to the appropriate handler.
func (c *IPCController) HandleCommand(ctx context.Context, command ipc.Command) ipc.Response {
	c.Logger.Debug(
		"Handling IPC command",
		zap.String("action", command.Action),
	)

	if handler, ok := c.Handlers[command.Action]; ok {
		return handler(ctx, command)
	}

	return ipc.Response{
		Success: false,
		Message: "unknown command: " + command.Action,
		Code:    ipc.CodeUnknownCommand,
	}
}

// UpdateConfig updates the stored config.
func (c *IPCController) UpdateConfig(cfg *config.Config) {
	if c.infoHandler != nil {
		c.infoHandler.UpdateConfig(cfg)
	}
}

// SetConfigFieldCallback sets the callback for runtime config field changes
// and propagates it to the info handler. Must be called after construction
// (e.g. from initializeIPCController) since the constructor's registerHandlers
// runs before the callback can be set.
func (c *IPCController) SetConfigFieldCallback(
	cb func(ctx context.Context, key, value string) error,
) {
	c.SetConfigField = cb
	if c.infoHandler != nil {
		c.infoHandler.setConfigField = cb
	}
}

// SetInfrastructure updates the infrastructure references on the controller
// and its info handler. This is called after event tap and IPC server are
// initialized (Phase 8), since the IPC controller is created earlier (Phase 7).
func (c *IPCController) SetInfrastructure(eventTap ports.EventTapPort, ipcServer ports.IPCPort) {
	c.EventTap = eventTap

	c.IPCServer = ipcServer
	if c.infoHandler != nil {
		c.infoHandler.eventTap = eventTap
		c.infoHandler.ipcServer = ipcServer
	}
}

// registerHandlers registers all command handlers by delegating to sub-controllers.
func (c *IPCController) registerHandlers(cfg *config.Config) {
	// Initialize handler components
	lifecycleHandler := NewIPCControllerLifecycle(c.AppState, c.Modes, c.Logger)
	modesHandler := NewIPCControllerModes(c.Modes, c.Logger)
	actionsHandler := NewIPCControllerActions(
		c.ActionService,
		c.ScrollService,
		c.Modes,
		c.AppState,
		c.Logger,
	)
	c.infoHandler = NewIPCControllerInfo(
		c.ConfigService,
		c.AppState,
		cfg,
		c.Modes,
		c.HintService,
		c.GridService,
		c.ActionService,
		c.ScrollService,
		c.System,
		c.EventTap,
		c.IPCServer,
		c.ReloadConfig,
		c.Logger,
		nil, // setConfigField — set below before RegisterHandlers
	)

	// Register handlers from each component
	lifecycleHandler.RegisterHandlers(c.Handlers)
	modesHandler.RegisterHandlers(c.Handlers)
	actionsHandler.RegisterHandlers(c.Handlers)

	c.infoHandler.RegisterHandlers(c.Handlers)

	// Register overlay handler
	overlayHandler := NewIPCControllerOverlay(c.AppState, c.Logger)
	overlayHandler.RegisterHandlers(c.Handlers)

	// Register scroll handler
	scrollHandler := NewIPCControllerScroll(c.AppState, c.ScrollService, c.Logger)
	scrollHandler.RegisterHandlers(c.Handlers)
}
