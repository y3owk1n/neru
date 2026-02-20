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
	if appState == nil {
		panic("appState cannot be nil")
	}

	if logger == nil {
		panic("logger cannot be nil")
	}

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

// IPCControllerModes handles mode-related IPC commands.
type IPCControllerModes struct {
	modes  *modes.Handler
	logger *zap.Logger // Reserved for future logging needs (maintains consistency with other IPC controllers)
}

// NewIPCControllerModes creates a new mode command handler.
func NewIPCControllerModes(modes *modes.Handler, logger *zap.Logger) *IPCControllerModes {
	return &IPCControllerModes{
		modes:  modes,
		logger: logger,
	}
}

// RegisterHandlers registers mode command handlers.
func (h *IPCControllerModes) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers["hints"] = h.handleHints
	handlers["grid"] = h.handleGrid
	handlers["recursive_grid"] = h.handleRecursiveGrid
	handlers["scroll"] = h.handleScroll
	handlers["idle"] = h.handleIdle
}

// modesUnavailableResponse returns a standardized response when modes handler is not available.
func (h *IPCControllerModes) modesUnavailableResponse() ipc.Response {
	return ipc.Response{
		Success: false,
		Message: "modes handler not available",
		Code:    ipc.CodeActionFailed,
	}
}

func (h *IPCControllerModes) handleHints(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 1 {
		action = &cmd.Args[1]
	}

	h.modes.ActivateModeWithAction(domain.ModeHints, action)

	return ipc.Response{Success: true, Message: "hints mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 1 {
		action = &cmd.Args[1]
	}

	h.modes.ActivateModeWithAction(domain.ModeGrid, action)

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleRecursiveGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 1 {
		action = &cmd.Args[1]
	}

	h.modes.ActivateModeWithAction(domain.ModeRecursiveGrid, action)

	return ipc.Response{Success: true, Message: "recursive-grid mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleScroll(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	h.modes.ActivateMode(domain.ModeScroll)

	return ipc.Response{Success: true, Message: "scroll mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	h.modes.ActivateMode(domain.ModeIdle)

	return ipc.Response{Success: true, Message: "idle mode activated", Code: ipc.CodeOK}
}

// IPCControllerOverlay handles overlay-related IPC commands.
type IPCControllerOverlay struct {
	appState *state.AppState
	logger   *zap.Logger
}

// NewIPCControllerOverlay creates a new overlay command handler.
func NewIPCControllerOverlay(appState *state.AppState, logger *zap.Logger) *IPCControllerOverlay {
	return &IPCControllerOverlay{
		appState: appState,
		logger:   logger,
	}
}

// RegisterHandlers registers overlay command handlers.
func (h *IPCControllerOverlay) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScreenShare] = h.handleToggleScreenShare
}

func (h *IPCControllerOverlay) handleToggleScreenShare(
	_ context.Context,
	_ ipc.Command,
) ipc.Response {
	currentState := h.appState.IsHiddenForScreenShare()
	newState := !currentState
	h.appState.SetHiddenForScreenShare(newState)

	status := "visible"
	if newState {
		status = "hidden"
	}

	return ipc.Response{
		Success: true,
		Message: "screen share visibility: " + status,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"hidden": newState},
	}
}
