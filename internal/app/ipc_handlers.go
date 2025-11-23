package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"go.uber.org/zap"
)

// handleIPCCommand processes IPC commands received from the CLI interface.
func (a *App) handleIPCCommand(ctx context.Context, cmd ipc.Command) ipc.Response {
	a.logger.Info(
		"Handling IPC command",
		zap.String("action", cmd.Action),
		zap.String("args", strings.Join(cmd.Args, ", ")),
	)

	if h, ok := a.cmdHandlers[cmd.Action]; ok {
		return h(ctx, cmd)
	}
	return ipc.Response{
		Success: false,
		Message: "unknown command: " + cmd.Action,
		Code:    ipc.CodeUnknownCommand,
	}
}

func (a *App) handlePing(_ context.Context, _ ipc.Command) ipc.Response {
	return ipc.Response{Success: true, Message: "pong", Code: ipc.CodeOK}
}

func (a *App) handleStart(_ context.Context, _ ipc.Command) ipc.Response {
	if a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already running",
			Code:    ipc.CodeAlreadyRunning,
		}
	}
	a.state.SetEnabled(true)
	return ipc.Response{Success: true, Message: "neru started", Code: ipc.CodeOK}
}

func (a *App) handleStop(_ context.Context, _ ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is already stopped",
			Code:    ipc.CodeNotRunning,
		}
	}
	a.state.SetEnabled(false)
	a.ExitMode()
	return ipc.Response{Success: true, Message: "neru stopped", Code: ipc.CodeOK}
}

func (a *App) handleHints(_ context.Context, cmd ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	if !a.config.Hints.Enabled {
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

	a.modes.ActivateModeWithAction(domain.ModeHints, action)

	return ipc.Response{Success: true, Message: "hint mode activated", Code: ipc.CodeOK}
}

func (a *App) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	if !a.config.Grid.Enabled {
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

	a.modes.ActivateModeWithAction(domain.ModeGrid, action)

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (a *App) handleAction(ctx context.Context, cmd ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
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

	cursorPos := infra.GetCurrentCursorPosition()

	for _, param := range params {
		var err error
		switch param {
		case "scroll":
			a.modes.StartInteractiveScroll()
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
			err = a.actionService.PerformAction(ctx, param, cursorPos)
		}

		if err != nil {
			a.logger.Error("Action failed",
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

func (a *App) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}
	a.ExitMode()
	return ipc.Response{Success: true, Message: "mode set to idle", Code: ipc.CodeOK}
}

func (a *App) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	cfgPath := a.resolveConfigPath()
	statusData := ipc.StatusData{
		Enabled: a.state.IsEnabled(),
		Mode:    domain.GetModeString(a.CurrentMode()),
		Config:  cfgPath,
	}
	return ipc.Response{Success: true, Data: statusData, Code: ipc.CodeOK}
}

func (a *App) handleConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if a.config == nil {
		return ipc.Response{Success: false, Message: "config unavailable", Code: ipc.CodeNotRunning}
	}
	return ipc.Response{Success: true, Data: a.config, Code: ipc.CodeOK}
}

func (a *App) handleReloadConfig(_ context.Context, _ ipc.Command) ipc.Response {
	if !a.state.IsEnabled() {
		return ipc.Response{
			Success: false,
			Message: "neru is not running",
			Code:    ipc.CodeNotRunning,
		}
	}

	configPath := a.ConfigPath
	if configPath == "" {
		configPath = config.FindConfigFile()
	}

	err := a.ReloadConfig(configPath)
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

func (a *App) handleHealth(ctx context.Context, _ ipc.Command) ipc.Response {
	// ctx is already available from argument
	healthStatus := a.hintService.Health(ctx)

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
	if a.config == nil {
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

func (a *App) handleMetrics(_ context.Context, _ ipc.Command) ipc.Response {
	if a.metrics == nil {
		return ipc.Response{
			Success: false,
			Message: "metrics collector not initialized",
			Code:    ipc.CodeActionFailed,
		}
	}

	snapshot := a.metrics.Snapshot()
	return ipc.Response{
		Success: true,
		Data:    snapshot,
		Code:    ipc.CodeOK,
	}
}

// resolveConfigPath determines the configuration file path for status reporting.
func (a *App) resolveConfigPath() string {
	cfgPath := a.ConfigPath

	if cfgPath == "" {
		// Fallback to the standard config path if daemon wasn't started with an explicit --config
		cfgPath = config.FindConfigFile()
	}

	var err error
	_, err = os.Stat(cfgPath)
	if os.IsNotExist(err) {
		return "No config file found, using default config without config file"
	}

	if strings.HasPrefix(cfgPath, "~") {
		var home string
		var err error
		home, err = os.UserHomeDir()
		if err == nil {
			cfgPath = filepath.Join(home, cfgPath[1:])
		}
	}
	var abs string
	var err2 error
	abs, err2 = filepath.Abs(cfgPath)
	if err2 == nil {
		cfgPath = abs
	}

	return cfgPath
}
