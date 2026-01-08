package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

// IPCControllerActions handles action-related IPC commands.
type IPCControllerActions struct {
	actionService *services.ActionService
	logger        *zap.Logger
}

// NewIPCControllerActions creates a new action command handler.
func NewIPCControllerActions(
	actionService *services.ActionService,
	logger *zap.Logger,
) *IPCControllerActions {
	return &IPCControllerActions{
		actionService: actionService,
		logger:        logger,
	}
}

// RegisterHandlers registers action command handlers.
func (h *IPCControllerActions) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers["action"] = h.handleAction
}

func (h *IPCControllerActions) handleAction(ctx context.Context, cmd ipc.Command) ipc.Response {
	if h.actionService == nil {
		return ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	if len(cmd.Args) == 0 {
		return ipc.Response{
			Success: false,
			Message: "action subcommand required (e.g., left_click, right_click)",
			Code:    ipc.CodeInvalidInput,
		}
	}

	actionName := cmd.Args[0]

	// Get current cursor position
	cursorPos, err := h.actionService.CursorPosition(ctx)
	if err != nil {
		h.logger.Error("Failed to get cursor position", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to get cursor position",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.logger.Info("Performing action via IPC",
		zap.String("action", actionName),
		zap.Int("x", cursorPos.X),
		zap.Int("y", cursorPos.Y),
	)

	err = h.actionService.PerformAction(ctx, actionName, cursorPos)
	if err != nil {
		h.logger.Error("Failed to perform action", zap.Error(err), zap.String("action", actionName))

		return ipc.Response{
			Success: false,
			Message: "failed to perform action: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: actionName + " performed",
		Code:    ipc.CodeOK,
	}
}
