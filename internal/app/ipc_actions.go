package app

import (
	"context"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
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
		xVal, yVal     int
		deltaX, deltaY int
		hasX, hasY     bool
		hasDX, hasDY   bool
		hasCenter      bool
		monitorName    string
		hasMonitor     bool
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

			xVal = val
			hasX = true

		case strings.HasPrefix(arg, "--y="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--y="))
			if err != nil {
				parseErr = true

				break
			}

			yVal = val
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
		case strings.HasPrefix(arg, "--monitor="):
			monitorName = strings.TrimPrefix(arg, "--monitor=")
			hasMonitor = monitorName != ""
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

	// Validation order matters:
	// 1. Reject coordinate flags on non-mouse-move actions.
	// 2. Reject --x/--y mixed with --dx/--dy (always invalid).
	// 3. Reject --center mixed with --dx/--dy (center uses --x/--y as offsets, not deltas).
	// 4. Reject --center on non-move_mouse actions.
	// 5. Require --x AND --y when --center is absent for move_mouse.
	// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.

	if !isMoveMouse && !isMoveMouseRelative && (hasX || hasY || hasDX || hasDY) {
		return ipc.Response{
			Success: false,
			Message: "--x/--y/--dx/--dy flags are only supported with move_mouse or move_mouse_relative",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if (isMoveMouse || isMoveMouseRelative) && (hasX || hasY) && (hasDX || hasDY) {
		return ipc.Response{
			Success: false,
			Message: "use either --x/--y or --dx/--dy, not both",
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

	if hasCenter && !isMoveMouse {
		return ipc.Response{
			Success: false,
			Message: "--center is only supported with move_mouse",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if hasMonitor && !hasCenter {
		return ipc.Response{
			Success: false,
			Message: "--monitor requires --center",
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

	if isMoveMouse && hasCenter {
		offsetX, offsetY := xVal, yVal

		if hasMonitor {
			h.logger.Info("Moving mouse to center of monitor via IPC",
				zap.String("monitor", monitorName),
				zap.Int("offsetX", offsetX),
				zap.Int("offsetY", offsetY),
			)

			err := h.actionService.MoveMouseToCenterOfMonitor(ctx, monitorName, offsetX, offsetY)
			if err != nil {
				h.logger.Error("Failed to move mouse to center of monitor", zap.Error(err))

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

		h.logger.Info("Moving mouse to center via IPC",
			zap.Int("offsetX", offsetX),
			zap.Int("offsetY", offsetY),
		)

		err := h.actionService.MoveMouseToCenter(ctx, offsetX, offsetY)
		if err != nil {
			h.logger.Error("Failed to move mouse to center", zap.Error(err))

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

	if isMoveMouseRelative {
		if !hasDX || !hasDY {
			return ipc.Response{
				Success: false,
				Message: "move_mouse_relative requires --dx and --dy flags",
				Code:    ipc.CodeInvalidInput,
			}
		}

		h.logger.Info("Moving mouse relative via IPC",
			zap.Int("dx", deltaX),
			zap.Int("dy", deltaY),
		)

		err := h.actionService.MoveMouseRelative(ctx, deltaX, deltaY, false)
		if err != nil {
			h.logger.Error("Failed to move mouse relative", zap.Error(err))

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

	h.logger.Info("Performing action via IPC",
		zap.String("action", actionName),
		zap.Int("x", xVal),
		zap.Int("y", yVal),
	)

	var err error
	switch actionName {
	case string(action.NameMoveMouse):
		err = h.actionService.MoveMouseTo(ctx, xVal, yVal, false)
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
