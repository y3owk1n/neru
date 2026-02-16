package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/appmetrics"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
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
	Config   *config.Config

	// Infrastructure
	Logger  *zap.Logger
	Metrics appmetrics.Collector

	// Mode management
	Modes *modes.Handler

	// Config path for status reporting
	ConfigPath string

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
	logger *zap.Logger,
	metricsCollector appmetrics.Collector,
	configPath string,
) *IPCController {
	ipcController := &IPCController{
		HintService:   hintService,
		GridService:   gridService,
		ActionService: actionService,
		ScrollService: scrollService,
		ConfigService: configService,
		AppState:      appState,
		Config:        config,
		Modes:         modesHandler,
		Logger:        logger,
		Metrics:       metricsCollector,
		ConfigPath:    configPath,
		Handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}

	// Register command handlers
	ipcController.RegisterHandlers()

	return ipcController
}

// HandleCommand routes an IPC command to the appropriate handler.
func (c *IPCController) HandleCommand(ctx context.Context, command ipc.Command) ipc.Response {
	c.Logger.Info(
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

// RegisterHandlers registers all command handlers by delegating to sub-controllers.
func (c *IPCController) RegisterHandlers() {
	// Initialize handler components
	lifecycleHandler := NewIPCControllerLifecycle(c.AppState, c.Modes, c.Logger)
	modesHandler := NewIPCControllerModes(c.Modes, c.Logger)
	actionsHandler := NewIPCControllerActions(c.ActionService, c.Logger)
	infoHandler := NewIPCControllerInfo(
		c.ConfigService,
		c.AppState,
		c.Config,
		c.Modes,
		c.HintService,
		c.GridService,
		c.ActionService,
		c.ScrollService,
		c.Metrics,
		c.ConfigPath,
		c.Logger,
	)

	// Register handlers from each component
	lifecycleHandler.RegisterHandlers(c.Handlers)
	modesHandler.RegisterHandlers(c.Handlers)
	actionsHandler.RegisterHandlers(c.Handlers)
	infoHandler.RegisterHandlers(c.Handlers)
}
