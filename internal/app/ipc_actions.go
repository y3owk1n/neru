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

// extractStringFlag extracts a string value from --flag=value or --flag value form.
// It returns the value, the updated index, and whether the extraction succeeded.
func extractStringFlag(rawArgs []string, idx int, prefix string) (string, int, bool) {
	arg := rawArgs[idx]
	if after, ok := strings.CutPrefix(arg, prefix+"="); ok {
		return after, idx, true
	}

	if arg == prefix {
		if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
			return rawArgs[idx+1], idx + 1, true
		}

		return "", idx, false
	}

	return "", idx, false
}

// extractIntFlag extracts an integer value from --flag=value or --flag value form.
// It returns the value, the updated index, and whether the extraction succeeded.
func extractIntFlag(rawArgs []string, idx int, prefix string) (int, int, bool) {
	s, newIdx, ok := extractStringFlag(rawArgs, idx, prefix)
	if !ok {
		return 0, newIdx, false
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, newIdx, false
	}

	return val, newIdx, true
}

// parseActionArgs parses flag arguments from an action IPC command.
// Supports both --flag=value and --flag value forms.
func parseActionArgs(rawArgs []string) (parsedActionArgs, bool) {
	var parsed parsedActionArgs

	parseErr := false
	for idx := 0; idx < len(rawArgs); idx++ {
		arg := rawArgs[idx]
		switch {
		case strings.HasPrefix(arg, "--modifier") && (arg == "--modifier" || arg[len("--modifier")] == '='):
			val, newIdx, ok := extractStringFlag(rawArgs, idx, "--modifier")
			idx = newIdx

			if !ok || val == "" {
				parseErr = true

				break
			}

			parsed.modifierStr = val
		case strings.HasPrefix(arg, "--x") && (arg == "--x" || arg[len("--x")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--x")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.xVal = val
			parsed.hasX = true
		case strings.HasPrefix(arg, "--y") && (arg == "--y" || arg[len("--y")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--y")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.yVal = val
			parsed.hasY = true
		case strings.HasPrefix(arg, "--dx") && (arg == "--dx" || arg[len("--dx")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--dx")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.deltaX = val
			parsed.hasDX = true
		case strings.HasPrefix(arg, "--dy") && (arg == "--dy" || arg[len("--dy")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--dy")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.deltaY = val
			parsed.hasDY = true
		case arg == "--center":
			parsed.hasCenter = true
		case strings.HasPrefix(arg, "--monitor") && (arg == "--monitor" || arg[len("--monitor")] == '='):
			val, newIdx, ok := extractStringFlag(rawArgs, idx, "--monitor")
			idx = newIdx

			if !ok || val == "" {
				parseErr = true

				break
			}

			parsed.monitorName = val
			parsed.hasMonitor = true
		default:
			if strings.HasPrefix(arg, "--") {
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
	// 2. Reject --modifier on non-mouse-button actions.
	// 3. Reject --x/--y mixed with --dx/--dy (always invalid).
	// 4. Reject --center mixed with --dx/--dy (center uses --x/--y as offsets, not deltas).
	// 5. Reject --center on non-move_mouse actions.
	// 6. Reject --monitor without --center (monitor targeting requires center mode).
	// 7. Require --x AND --y when --center is absent for move_mouse.
	// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.
	// Note: --monitor on non-move_mouse is caught by step 5 (if --center present) or step 6 (if absent).

	if !isMoveMouse && !isMoveMouseRelative &&
		(parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY) {
		return ipc.Response{
			Success: false,
			Message: "--x/--y/--dx/--dy flags are only supported with move_mouse or move_mouse_relative",
			Code:    ipc.CodeInvalidInput,
		}
	}

	isMouseButton := actionName == string(action.NameLeftClick) ||
		actionName == string(action.NameRightClick) ||
		actionName == string(action.NameMiddleClick) ||
		actionName == string(action.NameMouseDown) ||
		actionName == string(action.NameMouseUp)

	if modifiers != 0 && !isMouseButton {
		return ipc.Response{
			Success: false,
			Message: "--modifier is only supported with click and mouse button actions",
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
