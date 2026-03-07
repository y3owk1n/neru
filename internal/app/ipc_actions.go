package app

import (
	"context"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain/action"
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
		hasCenter        bool
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
		case arg == "--center":
			hasCenter = true
		}
	}

	if parseErr {
		return ipc.Response{
			Success: false,
			Message: "invalid coordinate value",
			Code:    ipc.CodeInvalidInput,
		}
	}

	isMoveMouse := actionName == string(action.NameMoveMouse)
	isMoveMouseRelative := actionName == string(action.NameMoveMouseRelative)

	if (isMoveMouse || isMoveMouseRelative) && (hasX || hasY) && (hasDX || hasDY) {
		return ipc.Response{
			Success: false,
			Message: "use either --x/--y or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && !hasCenter && (!hasX || !hasY) {
		return ipc.Response{
			Success: false,
			Message: "move_mouse requires --x and --y flags, or --center",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if hasCenter && (hasDX || hasDY) {
		return ipc.Response{
			Success: false,
			Message: "use either --center or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && hasCenter {
		screenBounds, err := h.actionService.ScreenBounds(ctx)
		if err != nil {
			h.logger.Error("Failed to get screen bounds", zap.Error(err))

			return ipc.Response{
				Success: false,
				Message: "failed to get screen bounds",
				Code:    ipc.CodeActionFailed,
			}
		}

		centerX := screenBounds.Min.X + screenBounds.Dx()/2 //nolint:mnd
		centerY := screenBounds.Min.Y + screenBounds.Dy()/2 //nolint:mnd
		targetX = centerX + targetX                         // targetX acts as offset (defaults to 0)
		targetY = centerY + targetY                         // targetY acts as offset (defaults to 0)
	}

	if isMoveMouseRelative {
		if !hasDX || !hasDY {
			return ipc.Response{
				Success: false,
				Message: "move_mouse_relative requires --dx and --dy flags",
				Code:    ipc.CodeInvalidInput,
			}
		}

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

	var err error
	switch actionName {
	case string(action.NameMoveMouse), string(action.NameMoveMouseRelative):
		err = h.actionService.MoveMouseTo(ctx, targetX, targetY, false)
	default:
		cursorPos, posErr := h.actionService.CursorPosition(ctx)
		if posErr != nil {
			h.logger.Error("Failed to get cursor position", zap.Error(posErr))

			return ipc.Response{
				Success: false,
				Message: "failed to get cursor position",
				Code:    ipc.CodeActionFailed,
			}
		}

		err = h.actionService.PerformActionAtPoint(ctx, actionName, cursorPos)
	}

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
