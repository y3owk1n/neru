package app

import (
	"context"
	"strconv"
	"strings"

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

	var (
		targetX, targetY int
		deltaX, deltaY   int
		hasX, hasY       bool
		hasDX, hasDY     bool
	)

	parseErr := false

	for _, arg := range cmd.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "--x="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--x="))
			if err != nil {
				parseErr = true

				break
			}

			targetX = val
			hasX = true

		case strings.HasPrefix(arg, "--y="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--y="))
			if err != nil {
				parseErr = true

				break
			}

			targetY = val
			hasY = true

		case strings.HasPrefix(arg, "--dx="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--dx="))
			if err != nil {
				parseErr = true

				break
			}

			deltaX = val
			hasDX = true

		case strings.HasPrefix(arg, "--dy="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--dy="))
			if err != nil {
				parseErr = true

				break
			}

			deltaY = val
			hasDY = true
		}
	}

	if parseErr {
		return ipc.Response{
			Success: false,
			Message: "invalid coordinate value",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if actionName == "move_mouse" && (!hasX || !hasY) {
		return ipc.Response{
			Success: false,
			Message: "move_mouse requires --x and --y flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if actionName == "move_mouse_relative" && (!hasDX || !hasDY) {
		return ipc.Response{
			Success: false,
			Message: "move_mouse_relative requires --dx and --dy flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if actionName == "move_mouse_relative" {
		cursorPos, err := h.actionService.CursorPosition(ctx)
		if err != nil {
			h.logger.Error("Failed to get cursor position", zap.Error(err))

			return ipc.Response{
				Success: false,
				Message: "failed to get cursor position",
				Code:    ipc.CodeActionFailed,
			}
		}

		targetX = cursorPos.X + deltaX
		targetY = cursorPos.Y + deltaY
	}

	h.logger.Info("Performing action via IPC",
		zap.String("action", actionName),
		zap.Int("x", targetX),
		zap.Int("y", targetY),
	)

	err := h.actionService.MoveMouseTo(ctx, targetX, targetY, false)
	if err != nil {
		h.logger.Error("Failed to move mouse", zap.Error(err), zap.String("action", actionName))

		return ipc.Response{
			Success: false,
			Message: "failed to move mouse: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: actionName + " performed",
		Code:    ipc.CodeOK,
	}
}
