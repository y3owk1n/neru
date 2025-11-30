package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

// IPCControllerLifecycle handles lifecycle-related IPC commands.
type IPCControllerLifecycle struct {
	appState *state.AppState
	modes    *modes.Handler
	logger   *zap.Logger
}

// NewIPCControllerLifecycle creates a new lifecycle command handler.
func NewIPCControllerLifecycle(
	appState *state.AppState,
	modes *modes.Handler,
	logger *zap.Logger,
) *IPCControllerLifecycle {
	return &IPCControllerLifecycle{
		appState: appState,
		modes:    modes,
		logger:   logger,
	}
}

// RegisterHandlers registers lifecycle command handlers.
func (h *IPCControllerLifecycle) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandPing] = h.handlePing
	handlers[domain.CommandStart] = h.handleStart
	handlers[domain.CommandStop] = h.handleStop
}

func (h *IPCControllerLifecycle) handlePing(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Debug("Received ping command")

	return ipc.Response{Success: true, Message: "pong", Code: ipc.CodeOK}
}

func (h *IPCControllerLifecycle) handleStart(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Info("Received start command")

	if h.appState.IsEnabled() {
		h.logger.Warn("Attempted to start neru when already running")

		return ipc.Response{
			Success: false,
			Message: "neru is already running",
			Code:    ipc.CodeAlreadyRunning,
		}
	}

	h.appState.SetEnabled(true)
	h.logger.Info("Neru started successfully", zap.Bool("enabled", true))

	return ipc.Response{Success: true, Message: "neru started", Code: ipc.CodeOK}
}

func (h *IPCControllerLifecycle) handleStop(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Info("Received stop command")

	if !h.appState.IsEnabled() {
		h.logger.Warn("Attempted to stop neru when already stopped")

		return ipc.Response{
			Success: false,
			Message: "neru is already stopped",
			Code:    ipc.CodeNotRunning,
		}
	}

	h.appState.SetEnabled(false)

	if h.modes != nil {
		h.modes.ExitMode()
	}

	h.logger.Info("Neru stopped successfully")

	return ipc.Response{Success: true, Message: "neru stopped", Code: ipc.CodeOK}
}
