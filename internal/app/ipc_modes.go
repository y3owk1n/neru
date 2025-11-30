package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

// IPCControllerModes handles mode-related IPC commands.
type IPCControllerModes struct {
	modes  *modes.Handler
	logger *zap.Logger
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
	handlers["scroll"] = h.handleScroll
	handlers[domain.CommandAction] = h.handleAction
	handlers["idle"] = h.handleIdle
}

func (h *IPCControllerModes) handleHints(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 0 {
		action = &cmd.Args[0]
	}

	h.modes.ActivateModeWithAction(domain.ModeHints, action)

	return ipc.Response{Success: true, Message: "hints mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// Extract action parameter if provided
	var action *string
	if len(cmd.Args) > 0 {
		action = &cmd.Args[0]
	}

	h.modes.ActivateModeWithAction(domain.ModeGrid, action)

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleScroll(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modes.ActivateMode(domain.ModeScroll)

	return ipc.Response{Success: true, Message: "scroll mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleAction(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// For action mode, just activate it (the original implementation uses ActionService)
	h.modes.ActivateMode(domain.ModeAction)

	return ipc.Response{Success: true, Message: "action mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modes.ActivateMode(domain.ModeIdle)

	return ipc.Response{Success: true, Message: "idle mode activated", Code: ipc.CodeOK}
}
