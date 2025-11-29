package app

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
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
	Metrics metrics.Collector

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
	metricsCollector metrics.Collector,
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

// RegisterHandlers registers all command handlers.
func (c *IPCController) RegisterHandlers() {
	c.Handlers[domain.CommandPing] = c.handlePing
	c.Handlers[domain.CommandStart] = c.handleStart
	c.Handlers[domain.CommandStop] = c.handleStop
	c.Handlers["hints"] = c.handleHints
	c.Handlers["grid"] = c.handleGrid
	c.Handlers["scroll"] = c.handleScroll
	c.Handlers[domain.CommandAction] = c.handleAction
	c.Handlers["idle"] = c.handleIdle
	c.Handlers[domain.CommandStatus] = c.handleStatus
	c.Handlers[domain.CommandConfig] = c.handleConfig
	c.Handlers[domain.CommandReloadConfig] = c.handleReloadConfig
	c.Handlers[domain.CommandHealth] = c.handleHealth
	c.Handlers[domain.CommandMetrics] = c.handleMetrics
}

func (c *IPCController) handlePing(_ context.Context, _ ipc.Command) ipc.Response {
	return ipc.Response{Success: true, Message: "pong", Code: ipc.CodeOK}
}

func (c *IPCController) handleStart(_ context.Context, _ ipc.Command) ipc.Response {
	if c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already running",
			Code:    ipc.CodeAlreadyRunning,
		}
	}

	c.AppState.SetEnabled(true)

	return ipc.Response{Success: true, Message: "neru started", Code: ipc.CodeOK}
}

func (c *IPCController) handleStop(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already stopped",
			Code:    ipc.CodeNotRunning,
		}
	}

	c.AppState.SetEnabled(false)

	if c.Modes != nil {
		c.Modes.ExitMode()
	}

	return ipc.Response{Success: true, Message: "neru stopped", Code: ipc.CodeOK}
}

func (c *IPCController) handleHints(_ context.Context, cmd ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	if !c.Config.Hints.Enabled {
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

	if c.Modes != nil {
		c.Modes.ActivateModeWithAction(domain.ModeHints, action)
	}

	return ipc.Response{Success: true, Message: "hint mode activated", Code: ipc.CodeOK}
}

func (c *IPCController) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	if !c.Config.Grid.Enabled {
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

	if c.Modes != nil {
		c.Modes.ActivateModeWithAction(domain.ModeGrid, action)
	}

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (c *IPCController) handleScroll(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	if c.Modes != nil {
		c.Modes.StartInteractiveScroll()
	}

	return ipc.Response{Success: true, Message: "scroll mode activated", Code: ipc.CodeOK}
}

func (c *IPCController) handleAction(ctx context.Context, cmd ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	params := cmd.Args
	if len(params) == 0 {
		// No args means enter action mode
		if c.Modes != nil {
			c.Modes.StartActionMode()
		}

		return ipc.Response{Success: true, Message: "action mode activated", Code: ipc.CodeOK}
	}

	cursorPos := accessibility.CurrentCursorPosition()

	for _, param := range params {
		var err error

		if !domain.IsKnownActionName(domain.ActionName(param)) {
			return ipc.Response{
				Success: false,
				Message: "unknown action: " + param,
				Code:    ipc.CodeInvalidInput,
			}
		}

		err = c.ActionService.PerformAction(ctx, param, cursorPos)
		if err != nil {
			c.Logger.Error("Action failed",
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
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	if c.Modes != nil {
		c.Modes.ExitMode()
	}

	return ipc.Response{Success: true, Message: "mode set to idle", Code: ipc.CodeOK}
}

func (c *IPCController) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	configPath := c.resolveConfigPath()

	modeString := "idle"
	if c.Modes != nil {
		modeString = c.Modes.CurrModeString()
	}

	statusData := ipc.StatusData{
		Enabled: c.AppState.IsEnabled(),
		Mode:    modeString,
		Config:  configPath,
	}

	return ipc.Response{Success: true, Data: statusData, Code: ipc.CodeOK}
}

func (c *IPCController) handleConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if c.Config == nil {
		return ipc.Response{Success: false, Message: "config unavailable", Code: ipc.CodeNotRunning}
	}

	return ipc.Response{Success: true, Data: c.Config, Code: ipc.CodeOK}
}

func (c *IPCController) handleReloadConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if !c.AppState.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	configPath := c.ConfigPath
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	reloadConfigErr := c.ConfigService.ReloadConfig(configPath)
	if reloadConfigErr != nil {
		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf("failed to reload config: %v", reloadConfigErr),
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
	healthStatus := c.HintService.Health(ctx)

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
	if c.Config == nil {
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
	if c.Metrics == nil {
		return ipc.Response{
			Success: false,
			Message: "metrics collector not initialized",
			Code:    ipc.CodeActionFailed,
		}
	}

	snapshot := c.Metrics.Snapshot()

	return ipc.Response{
		Success: true,
		Data:    snapshot,
		Code:    ipc.CodeOK,
	}
}

// resolveConfigPath determines the configuration file path for status reporting.
func (c *IPCController) resolveConfigPath() string {
	return c.ConfigService.GetConfigPath()
}
