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

// parsedActionArgs holds the parsed arguments from an action IPC command.
type parsedActionArgs struct {
	xVal, yVal     int
	deltaX, deltaY int
	hasX, hasY     bool
	hasDX, hasDY   bool
	hasCenter      bool
	monitorName    string
	hasMonitor     bool
	modifierStr    string
}

// parseActionArgs parses flag arguments from an action IPC command.
// Supports both --flag=value and --flag value forms.
func parseActionArgs(rawArgs []string) (parsedActionArgs, bool) {
	var parsed parsedActionArgs

	parseErr := false
	for idx := 0; idx < len(rawArgs); idx++ {
		arg := rawArgs[idx]
		switch {
		case strings.HasPrefix(arg, "--modifier="):
			parsed.modifierStr = strings.TrimPrefix(arg, "--modifier=")
		case arg == "--modifier":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++
				parsed.modifierStr = rawArgs[idx]
			} else {
				parseErr = true
			}
		case strings.HasPrefix(arg, "--x="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--x="))
			if err != nil {
				parseErr = true

				break
			}

			parsed.xVal = val
			parsed.hasX = true
		case arg == "--x":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++

				val, err := strconv.Atoi(rawArgs[idx])
				if err != nil {
					parseErr = true

					break
				}

				parsed.xVal = val
				parsed.hasX = true
			} else {
				parseErr = true
			}
		case strings.HasPrefix(arg, "--y="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--y="))
			if err != nil {
				parseErr = true

				break
			}

			parsed.yVal = val
			parsed.hasY = true
		case arg == "--y":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++

				val, err := strconv.Atoi(rawArgs[idx])
				if err != nil {
					parseErr = true

					break
				}

				parsed.yVal = val
				parsed.hasY = true
			} else {
				parseErr = true
			}
		case strings.HasPrefix(arg, "--dx="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--dx="))
			if err != nil {
				parseErr = true

				break
			}

			parsed.deltaX = val
			parsed.hasDX = true
		case arg == "--dx":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++

				val, err := strconv.Atoi(rawArgs[idx])
				if err != nil {
					parseErr = true

					break
				}

				parsed.deltaX = val
				parsed.hasDX = true
			} else {
				parseErr = true
			}
		case strings.HasPrefix(arg, "--dy="):
			val, err := strconv.Atoi(strings.TrimPrefix(arg, "--dy="))
			if err != nil {
				parseErr = true

				break
			}

			parsed.deltaY = val
			parsed.hasDY = true
		case arg == "--dy":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++

				val, err := strconv.Atoi(rawArgs[idx])
				if err != nil {
					parseErr = true

					break
				}

				parsed.deltaY = val
				parsed.hasDY = true
			} else {
				parseErr = true
			}
		case arg == "--center":
			parsed.hasCenter = true
		case strings.HasPrefix(arg, "--monitor="):
			parsed.monitorName = strings.TrimPrefix(arg, "--monitor=")
			parsed.hasMonitor = parsed.monitorName != ""
		case arg == "--monitor":
			if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
				idx++
				parsed.monitorName = rawArgs[idx]
				parsed.hasMonitor = parsed.monitorName != ""
			} else {
				parseErr = true
			}
		}
	}

	return parsed, parseErr
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

	parsed, parseErr := parseActionArgs(cmd.Args[1:])
	if parseErr {
		return ipc.Response{
			Success: false,
			Message: "invalid or missing flag value",
			Code:    ipc.CodeInvalidInput,
		}
	}

	modifiers, modErr := action.ParseModifiers(parsed.modifierStr)
	if modErr != nil {
		return ipc.Response{
			Success: false,
			Message: modErr.Error(),
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
	// 5. Reject --monitor without --center (monitor targeting requires center mode).
	// 6. Require --x AND --y when --center is absent for move_mouse.
	// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.
	// Note: --monitor on non-move_mouse is caught by step 4 (if --center present) or step 5 (if absent).

	if !isMoveMouse && !isMoveMouseRelative &&
		(parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY) {
		return ipc.Response{
			Success: false,
			Message: "--x/--y/--dx/--dy flags are only supported with move_mouse or move_mouse_relative",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if (isMoveMouse || isMoveMouseRelative) && modifiers != 0 {
		return ipc.Response{
			Success: false,
			Message: "--modifier is not supported with move_mouse or move_mouse_relative",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if (isMoveMouse || isMoveMouseRelative) && (parsed.hasX || parsed.hasY) &&
		(parsed.hasDX || parsed.hasDY) {
		return ipc.Response{
			Success: false,
			Message: "use either --x/--y or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasCenter && (parsed.hasDX || parsed.hasDY) {
		return ipc.Response{
			Success: false,
			Message: "use either --center or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasCenter && !isMoveMouse {
		return ipc.Response{
			Success: false,
			Message: "--center is only supported with move_mouse",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasMonitor && !parsed.hasCenter {
		return ipc.Response{
			Success: false,
			Message: "--monitor requires --center",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && !parsed.hasCenter && (!parsed.hasX || !parsed.hasY) {
		return ipc.Response{
			Success: false,
			Message: "move_mouse requires --x and --y flags, or --center",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && parsed.hasCenter {
		offsetX, offsetY := parsed.xVal, parsed.yVal

		if parsed.hasMonitor {
			h.logger.Info("Moving mouse to center of monitor via IPC",
				zap.String("monitor", parsed.monitorName),
				zap.Int("offsetX", offsetX),
				zap.Int("offsetY", offsetY),
			)

			err := h.actionService.MoveMouseToCenterOfMonitor(
				ctx,
				parsed.monitorName,
				offsetX,
				offsetY,
			)
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
		if !parsed.hasDX || !parsed.hasDY {
			return ipc.Response{
				Success: false,
				Message: "move_mouse_relative requires --dx and --dy flags",
				Code:    ipc.CodeInvalidInput,
			}
		}

		h.logger.Info("Moving mouse relative via IPC",
			zap.Int("dx", parsed.deltaX),
			zap.Int("dy", parsed.deltaY),
		)

		err := h.actionService.MoveMouseRelative(ctx, parsed.deltaX, parsed.deltaY, false)
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
		zap.Int("x", parsed.xVal),
		zap.Int("y", parsed.yVal),
	)

	var err error
	switch actionName {
	case string(action.NameMoveMouse):
		err = h.actionService.MoveMouseTo(ctx, parsed.xVal, parsed.yVal, false)
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

		err = h.actionService.PerformActionAtPoint(ctx, actionName, cursorPos, modifiers)
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
