package app

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/metrics"
	"go.uber.org/zap"
)

// IPCController handles IPC command routing and execution.
type IPCController struct {
	// Services
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	configService *config.Service

	// State
	state  *state.AppState
	config *config.Config

	// Infrastructure
	logger  *zap.Logger
	metrics *metrics.Collector

	// Mode management
	modes *modes.Handler

	// Config path for status reporting
	configPath string

	// Command handlers map
	handlers map[string]func(context.Context, ipc.Command) ipc.Response
}

// NewIPCController creates a new IPC controller with the given dependencies.
func NewIPCController(
	hintService *services.HintService,
	gridService *services.GridService,
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	configService *config.Service,
	appState *state.AppState,
	cfg *config.Config,
	modesHandler *modes.Handler,
	logger *zap.Logger,
	metricsCollector *metrics.Collector,
	configPath string,
) *IPCController {
	ctrl := &IPCController{
		hintService:   hintService,
		gridService:   gridService,
		actionService: actionService,
		scrollService: scrollService,
		configService: configService,
		state:         appState,
		config:        cfg,
		modes:         modesHandler,
		logger:        logger,
		metrics:       metricsCollector,
		configPath:    configPath,
		handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}

	// Register command handlers
	ctrl.registerHandlers()

	return ctrl
}

// HandleCommand routes an IPC command to the appropriate handler.
func (c *IPCController) HandleCommand(ctx context.Context, cmd ipc.Command) ipc.Response {
	c.logger.Info(
		"Handling IPC command",
		zap.String("action", cmd.Action),
	)

	if handler, ok := c.handlers[cmd.Action]; ok {
		return handler(ctx, cmd)
	}

	return ipc.Response{
		Success: false,
		Message: "unknown command: " + cmd.Action,
		Code:    ipc.CodeUnknownCommand,
	}
}

// registerHandlers registers all command handlers.
func (c *IPCController) registerHandlers() {
	c.handlers[domain.CommandPing] = c.handlePing
	c.handlers[domain.CommandStart] = c.handleStart
	c.handlers[domain.CommandStop] = c.handleStop
	c.handlers["hints"] = c.handleHints
	c.handlers["grid"] = c.handleGrid
	c.handlers[domain.CommandAction] = c.handleAction
	c.handlers["idle"] = c.handleIdle
	c.handlers[domain.CommandStatus] = c.handleStatus
	c.handlers[domain.CommandConfig] = c.handleConfig
	c.handlers[domain.CommandReloadConfig] = c.handleReloadConfig
	c.handlers[domain.CommandHealth] = c.handleHealth
	c.handlers[domain.CommandMetrics] = c.handleMetrics
}

func (c *IPCController) handlePing(_ context.Context, _ ipc.Command) ipc.Response {
	return ipc.Response{Success: true, Message: "pong", Code: ipc.CodeOK}
}

func (c *IPCController) handleStart(_ context.Context, _ ipc.Command) ipc.Response {
	if c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already running",
			Code:    ipc.CodeAlreadyRunning,
		}
	}
	c.state.SetEnabled(true)
	return ipc.Response{Success: true, Message: "neru started", Code: ipc.CodeOK}
}

func (c *IPCController) handleStop(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already stopped",
			Code:    ipc.CodeNotRunning,
		}
	}
	c.state.SetEnabled(false)
	if c.modes != nil {
		c.modes.ExitMode()
	}
	return ipc.Response{Success: true, Message: "neru stopped", Code: ipc.CodeOK}
}

func (c *IPCController) handleHints(_ context.Context, cmd ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	if !c.config.Hints.Enabled {
		return ipc.Response{
			Success: false,
			Message: "hints mode is disabled by config",
			Code:    ipc.CodeModeDisabled,
		}
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 1 {
		action = &cmd.Args[1]
	}

	if c.modes != nil {
		c.modes.ActivateModeWithAction(domain.ModeHints, action)
	}

	return ipc.Response{Success: true, Message: "hint mode activated", Code: ipc.CodeOK}
}

func (c *IPCController) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	if !c.config.Grid.Enabled {
		return ipc.Response{
			Success: false,
			Message: "grid mode is disabled by config",
			Code:    ipc.CodeModeDisabled,
		}
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 1 {
		action = &cmd.Args[1]
	}

	if c.modes != nil {
		c.modes.ActivateModeWithAction(domain.ModeGrid, action)
	}

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (c *IPCController) handleAction(ctx context.Context, cmd ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	params := cmd.Args
	if len(params) == 0 {
		return ipc.Response{
			Success: false,
			Message: "no action specified",
			Code:    ipc.CodeInvalidInput,
		}
	}

	cursorPos := accessibility.GetCurrentCursorPosition()

	for _, param := range params {
		var err error
		switch param {
		case "scroll":
			if c.modes != nil {
				c.modes.StartInteractiveScroll()
			}
			return ipc.Response{Success: true, Message: "scroll mode activated", Code: ipc.CodeOK}
		default:
			if !domain.IsKnownActionName(domain.ActionName(param)) {
				return ipc.Response{
					Success: false,
					Message: "unknown action: " + param,
					Code:    ipc.CodeInvalidInput,
				}
			}
			// Use ActionService
			// ctx is already available from argument
			err = c.actionService.PerformAction(ctx, param, cursorPos)
		}

		if err != nil {
			c.logger.Error("Action failed",
				zap.Error(err),
				zap.String("action", param),
				zap.String("point", fmt.Sprintf("%+v", cursorPos)))
			return ipc.Response{
				Success: false,
				Message: "action failed: " + err.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}
	}

	return ipc.Response{Success: true, Message: "action performed at cursor", Code: ipc.CodeOK}
}

func (c *IPCController) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	if c.modes != nil {
		c.modes.ExitMode()
	}
	return ipc.Response{Success: true, Message: "mode set to idle", Code: ipc.CodeOK}
}

func (c *IPCController) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	cfgPath := c.resolveConfigPath()
	modeString := "idle"
	if c.modes != nil {
		modeString = c.modes.GetCurrModeString()
	}
	statusData := ipc.StatusData{
		Enabled: c.state.IsEnabled(),
		Mode:    modeString,
		Config:  cfgPath,
	}
	return ipc.Response{Success: true, Data: statusData, Code: ipc.CodeOK}
}

func (c *IPCController) handleConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if c.config == nil {
		return ipc.Response{Success: false, Message: "config unavailable", Code: ipc.CodeNotRunning}
	}
	return ipc.Response{Success: true, Data: c.config, Code: ipc.CodeOK}
}

func (c *IPCController) handleReloadConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	configPath := c.configPath
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	err := c.configService.ReloadConfig(configPath)
	if err != nil {
		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf("failed to reload config: %v", err),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "configuration reloaded successfully",
		Code:    ipc.CodeOK,
	}
}

func (c *IPCController) handleHealth(ctx context.Context, _ ipc.Command) ipc.Response {
	// ctx is already available from argument
	healthStatus := c.hintService.Health(ctx)

	status := make(map[string]string)
	allHealthy := true

	for component, err := range healthStatus {
		if err != nil {
			status[component] = fmt.Sprintf("unhealthy: %v", err)
			allHealthy = false
		} else {
			status[component] = "healthy"
		}
	}

	// Check Config Service (if it has health check, otherwise skip or check file)
	// We can check if config is loaded
	if c.config == nil {
		status["config"] = "unhealthy: config not loaded"
		allHealthy = false
	} else {
		status["config"] = "healthy"
	}

	if !allHealthy {
		return ipc.Response{
			Success: false,
			Message: "some components are unhealthy",
			Code:    ipc.CodeActionFailed,
			Data:    status,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "all systems operational",
		Code:    ipc.CodeOK,
		Data:    status,
	}
}

func (c *IPCController) handleMetrics(_ context.Context, _ ipc.Command) ipc.Response {
	if c.metrics == nil {
		return ipc.Response{
			Success: false,
			Message: "metrics collector not initialized",
			Code:    ipc.CodeActionFailed,
		}
	}

	snapshot := c.metrics.Snapshot()
	return ipc.Response{
		Success: true,
		Data:    snapshot,
		Code:    ipc.CodeOK,
	}
}

// resolveConfigPath determines the configuration file path for status reporting.
func (c *IPCController) resolveConfigPath() string {
	return c.configService.GetConfigPath()
}
