package app

import (
	"context"
	"image"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// IPCControllerActions handles action-related IPC commands.
type IPCControllerActions struct {
	actionService *services.ActionService
	scrollService *services.ScrollService
	modesHandler  *modes.Handler
	appState      *state.AppState
	logger        *zap.Logger

	savedCursorMu      sync.RWMutex
	savedCursorPos     image.Point
	savedCursorPresent bool
}

const modeExitPollInterval = 10 * time.Millisecond

// modeExitTimeout is the maximum time wait_for_mode_exit will block before
// giving up. This prevents goroutine leaks when the mode never exits (e.g.
// the user abandons the workflow). 5 minutes is generous for any interactive
// mode session.
const modeExitTimeout = 5 * time.Minute

// NewIPCControllerActions creates a new action command handler.
func NewIPCControllerActions(
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	modesHandler *modes.Handler,
	appState *state.AppState,
	logger *zap.Logger,
) *IPCControllerActions {
	return &IPCControllerActions{
		actionService: actionService,
		scrollService: scrollService,
		modesHandler:  modesHandler,
		appState:      appState,
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
	useSelection   bool
	useBare        bool
	monitorName    string
	hasMonitor     bool
	modifierStr    string
}

func shouldClearSelectionAfterMoveMouse(parsed parsedActionArgs, targetsSelection bool) bool {
	if targetsSelection {
		return false
	}

	return (parsed.hasX && parsed.hasY) ||
		parsed.hasCenter ||
		(parsed.hasDX && parsed.hasDY) ||
		parsed.useBare
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
		case arg == "--selection":
			parsed.useSelection = true
		case arg == "--bare":
			parsed.useBare = true
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

	// Handle scroll sub-actions (scroll_up, scroll_down, etc.)
	// These only require scrollService, so dispatch before the actionService nil check.
	if action.IsScrollSubAction(actionName) {
		return h.handleScrollAction(ctx, actionName, parsed)
	}

	if action.IsResetAction(actionName) {
		return h.handleResetAction()
	}

	if action.IsBackspaceAction(actionName) {
		return h.handleBackspaceAction()
	}

	if action.IsWaitForModeExitAction(actionName) {
		return h.handleWaitForModeExitAction(ctx, parsed)
	}

	if action.IsSaveCursorPosAction(actionName) {
		return h.handleSaveCursorPosAction(ctx, parsed)
	}

	if action.IsRestoreCursorPosAction(actionName) {
		return h.handleRestoreCursorPosAction(ctx, parsed)
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
	isMouseButton := actionName == string(action.NameLeftClick) ||
		actionName == string(action.NameRightClick) ||
		actionName == string(action.NameMiddleClick) ||
		actionName == string(action.NameMouseDown) ||
		actionName == string(action.NameMouseUp)
	isPointTargetedAction := isMoveMouse || isMouseButton

	// Validation order matters:
	// 1. Reject coordinate flags on non-mouse-move actions.
	// 2. Reject --modifier on non-mouse-button actions.
	// 3. Reject --x/--y mixed with --dx/--dy (always invalid).
	// 4. Reject --center mixed with --dx/--dy (center uses --x/--y as offsets, not deltas).
	// 5. Reject --center on non-move_mouse actions.
	// 6. Reject --monitor without --center (monitor targeting requires center mode).
	// 7. Reject --selection mixed with explicit move targeting.
	// 8. Require --x AND --y when --center is absent for move_mouse.
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

	if modifiers != 0 && !isMouseButton {
		return ipc.Response{
			Success: false,
			Message: "--modifier is only supported with click and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	// Merge sticky modifiers AFTER the explicit --modifier validation above,
	// so that active sticky modifiers don't cause false rejection of
	// non-click actions like move_mouse or move_mouse_relative.
	if h.modesHandler != nil {
		stickyMods := h.modesHandler.StickyModifiers()
		modifiers |= stickyMods
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

	if parsed.useSelection && parsed.useBare {
		return ipc.Response{
			Success: false,
			Message: "--selection and --bare cannot be used together",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection && (!isMoveMouse && !isMouseButton) {
		return ipc.Response{
			Success: false,
			Message: "--selection is only supported with move_mouse, scroll, and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useBare && !isPointTargetedAction {
		return ipc.Response{
			Success: false,
			Message: "--bare is only supported with move_mouse, scroll, and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection &&
		(parsed.hasCenter || parsed.hasMonitor || parsed.hasX || parsed.hasY) {
		return ipc.Response{
			Success: false,
			Message: "--selection cannot be combined with --x, --y, --center, or --monitor",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && !parsed.hasCenter &&
		!parsed.useSelection &&
		((parsed.hasX && !parsed.hasY) || (!parsed.hasX && parsed.hasY)) {
		return ipc.Response{
			Success: false,
			Message: "move_mouse requires both --x and --y when using absolute coordinates",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && parsed.hasCenter {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: "action service not available",
				Code:    ipc.CodeActionFailed,
			}
		}

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

			if h.modesHandler != nil &&
				shouldClearSelectionAfterMoveMouse(parsed, false) {
				h.modesHandler.ClearCurrentSelectionPoint()
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

		if h.modesHandler != nil &&
			shouldClearSelectionAfterMoveMouse(parsed, false) {
			h.modesHandler.ClearCurrentSelectionPoint()
		}

		return ipc.Response{
			Success: true,
			Message: actionName + " performed",
			Code:    ipc.CodeOK,
		}
	}

	if isMoveMouseRelative {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: "action service not available",
				Code:    ipc.CodeActionFailed,
			}
		}

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

		err := h.actionService.MoveMouseRelative(ctx, parsed.deltaX, parsed.deltaY, true)
		if err != nil {
			h.logger.Error("Failed to move mouse relative", zap.Error(err))

			return ipc.Response{
				Success: false,
				Message: "failed to perform action: " + err.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}

		if h.modesHandler != nil {
			if shouldClearSelectionAfterMoveMouse(parsed, false) {
				h.modesHandler.ClearCurrentSelectionPoint()
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

	var (
		err     error
		errResp *ipc.Response
	)

	if actionName == string(action.NameMoveMouse) {
		errResp, err = h.handleMoveMouseAction(ctx, parsed)
	} else {
		errResp, err = h.handlePointTargetedAction(ctx, actionName, parsed, modifiers)
	}

	if errResp != nil {
		return *errResp
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

func (h *IPCControllerActions) handleMoveMouseAction(
	ctx context.Context,
	parsed parsedActionArgs,
) (*ipc.Response, error) {
	if parsed.hasX && parsed.hasY {
		if h.actionService == nil {
			return &ipc.Response{
				Success: false,
				Message: "action service not available",
				Code:    ipc.CodeActionFailed,
			}, nil
		}

		moveErr := h.actionService.MoveMouseTo(ctx, parsed.xVal, parsed.yVal, false)
		if moveErr == nil &&
			h.modesHandler != nil &&
			shouldClearSelectionAfterMoveMouse(parsed, false) {
			h.modesHandler.ClearCurrentSelectionPoint()
		}

		return nil, moveErr
	}

	if parsed.useBare && h.actionService == nil {
		return &ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}, nil
	}

	targetPoint, pointErrResp := h.resolveMoveMousePoint(ctx, parsed)
	if pointErrResp != nil {
		return pointErrResp, nil
	}

	targetsSelection := parsed.useSelection
	if !targetsSelection && !parsed.useBare {
		if selectionPoint, ok := h.currentSelectionPoint(); ok &&
			targetPoint == selectionPoint {
			targetsSelection = true
		}
	}

	if h.actionService == nil {
		return &ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}, nil
	}

	moveErr := h.actionService.MoveCursorToPointAndWait(ctx, targetPoint)
	if moveErr == nil &&
		h.modesHandler != nil &&
		shouldClearSelectionAfterMoveMouse(parsed, targetsSelection) {
		h.modesHandler.ClearCurrentSelectionPoint()
	}

	return nil, moveErr
}

func (h *IPCControllerActions) handlePointTargetedAction(
	ctx context.Context,
	actionName string,
	parsed parsedActionArgs,
	modifiers action.Modifiers,
) (*ipc.Response, error) {
	if h.actionService == nil {
		return &ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}, nil
	}

	targetPoint, pointErrResp := h.resolveMouseActionPoint(ctx, parsed)
	if pointErrResp != nil {
		return pointErrResp, nil
	}

	targetsSelection := parsed.useSelection
	if !targetsSelection && !parsed.useBare {
		if selectionPoint, ok := h.currentSelectionPoint(); ok &&
			targetPoint == selectionPoint {
			targetsSelection = true
		}
	}

	if targetsSelection {
		moveErr := h.actionService.MoveCursorToPointAndWait(ctx, targetPoint)
		if moveErr != nil {
			h.logger.Error("Failed to move cursor to mode selection", zap.Error(moveErr))

			return &ipc.Response{
				Success: false,
				Message: "failed to perform action: " + moveErr.Error(),
				Code:    ipc.CodeActionFailed,
			}, nil
		}
	}

	return nil, h.actionService.PerformActionAtPoint(ctx, actionName, targetPoint, modifiers)
}

func (h *IPCControllerActions) resolveMoveMousePoint(
	ctx context.Context,
	parsed parsedActionArgs,
) (image.Point, *ipc.Response) {
	if parsed.useSelection {
		return h.resolveSelectionPoint()
	}

	if parsed.useBare {
		return h.resolveCurrentCursorPoint(ctx)
	}

	if targetPoint, ok := h.currentSelectionPoint(); ok {
		return targetPoint, nil
	}

	return image.Point{}, &ipc.Response{
		Success: false,
		Message: "move_mouse requires --x and --y flags, --center, active selection, or --bare",
		Code:    ipc.CodeInvalidInput,
	}
}

func (h *IPCControllerActions) handleResetAction() ipc.Response {
	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modesHandler.ResetCurrentMode()

	return ipc.Response{Success: true, Message: "mode reset", Code: ipc.CodeOK}
}

func (h *IPCControllerActions) handleBackspaceAction() ipc.Response {
	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: "modes handler not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modesHandler.BackspaceCurrentMode()

	return ipc.Response{Success: true, Message: "mode backspace", Code: ipc.CodeOK}
}

func hasUnsupportedFlags(parsed parsedActionArgs) bool {
	return parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.hasMonitor || parsed.modifierStr != "" ||
		parsed.useSelection || parsed.useBare
}

func (h *IPCControllerActions) resolveMouseActionPoint(
	ctx context.Context,
	parsed parsedActionArgs,
) (image.Point, *ipc.Response) {
	if parsed.useSelection {
		return h.resolveSelectionPoint()
	}

	if !parsed.useBare {
		if targetPoint, ok := h.currentSelectionPoint(); ok {
			return targetPoint, nil
		}
	}

	return h.resolveCurrentCursorPoint(ctx)
}

func (h *IPCControllerActions) resolveCurrentCursorPoint(
	ctx context.Context,
) (image.Point, *ipc.Response) {
	cursorPos, posErr := h.actionService.CursorPosition(ctx)
	if posErr != nil {
		h.logger.Error("Failed to get cursor position", zap.Error(posErr))

		return image.Point{}, &ipc.Response{
			Success: false,
			Message: "failed to get cursor position",
			Code:    ipc.CodeActionFailed,
		}
	}

	return cursorPos, nil
}

func (h *IPCControllerActions) resolveSelectionPoint() (image.Point, *ipc.Response) {
	if h.modesHandler == nil {
		return image.Point{}, &ipc.Response{
			Success: false,
			Message: "--selection requires an active mode selection",
			Code:    ipc.CodeInvalidInput,
		}
	}

	targetPoint, ok := h.modesHandler.CurrentSelectionPoint()
	if !ok {
		return image.Point{}, &ipc.Response{
			Success: false,
			Message: "--selection requires an active mode selection",
			Code:    ipc.CodeInvalidInput,
		}
	}

	return targetPoint, nil
}

func (h *IPCControllerActions) currentSelectionPoint() (image.Point, bool) {
	if h.modesHandler == nil {
		return image.Point{}, false
	}

	return h.modesHandler.CurrentSelectionPoint()
}

func (h *IPCControllerActions) handleWaitForModeExitAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if hasUnsupportedFlags(parsed) {
		return ipc.Response{
			Success: false,
			Message: "wait_for_mode_exit does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.appState == nil {
		return ipc.Response{
			Success: false,
			Message: "app state not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	deadline := time.After(modeExitTimeout)

	ticker := time.NewTicker(modeExitPollInterval)
	defer ticker.Stop()

	for h.appState.CurrentMode() != domain.ModeIdle {
		select {
		case <-ctx.Done():
			return ipc.Response{
				Success: false,
				Message: "wait_for_mode_exit canceled: " + ctx.Err().Error(),
				Code:    ipc.CodeActionFailed,
			}
		case <-deadline:
			return ipc.Response{
				Success: false,
				Message: "wait_for_mode_exit timed out after " + modeExitTimeout.String(),
				Code:    ipc.CodeActionFailed,
			}
		case <-ticker.C:
		}
	}

	return ipc.Response{Success: true, Message: "mode exited", Code: ipc.CodeOK}
}

func (h *IPCControllerActions) handleSaveCursorPosAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if hasUnsupportedFlags(parsed) {
		return ipc.Response{
			Success: false,
			Message: "save_cursor_pos does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.actionService == nil {
		return ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	pos, posErr := h.actionService.CursorPosition(ctx)
	if posErr != nil {
		return ipc.Response{
			Success: false,
			Message: "failed to capture cursor position: " + posErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	h.savedCursorMu.Lock()
	h.savedCursorPos = pos
	h.savedCursorPresent = true
	h.savedCursorMu.Unlock()

	return ipc.Response{Success: true, Message: "cursor position saved", Code: ipc.CodeOK}
}

func (h *IPCControllerActions) handleRestoreCursorPosAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if hasUnsupportedFlags(parsed) {
		return ipc.Response{
			Success: false,
			Message: "restore_cursor_pos does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.actionService == nil {
		return ipc.Response{
			Success: false,
			Message: "action service not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.savedCursorMu.RLock()
	initialPos := h.savedCursorPos
	present := h.savedCursorPresent
	h.savedCursorMu.RUnlock()

	if !present {
		return ipc.Response{Success: true, Message: "no saved cursor position", Code: ipc.CodeOK}
	}

	moveErr := h.actionService.MoveCursorToPoint(ctx, initialPos)
	if moveErr != nil {
		return ipc.Response{
			Success: false,
			Message: "failed to restore cursor position: " + moveErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	h.savedCursorMu.Lock()
	h.savedCursorPresent = false
	h.savedCursorMu.Unlock()

	return ipc.Response{Success: true, Message: "cursor restored", Code: ipc.CodeOK}
}

// handleScrollAction dispatches a scroll sub-action (scroll_up, page_down, etc.)
// to the ScrollService.
func (h *IPCControllerActions) handleScrollAction(
	ctx context.Context,
	actionName string,
	parsed parsedActionArgs,
) ipc.Response {
	if h.scrollService == nil {
		return ipc.Response{
			Success: false,
			Message: "scroll service not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// Reject flags that are not applicable to scroll actions.
	if parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.hasMonitor || parsed.modifierStr != "" {
		return ipc.Response{
			Success: false,
			Message: "scroll actions do not support --x/--y/--dx/--dy/--center/--monitor/--modifier flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection && parsed.useBare {
		return ipc.Response{
			Success: false,
			Message: "--selection and --bare cannot be used together",
			Code:    ipc.CodeInvalidInput,
		}
	}

	direction, amount, ok := scrollActionMapping(actionName)
	if !ok {
		return ipc.Response{
			Success: false,
			Message: "unknown scroll action: " + actionName,
			Code:    ipc.CodeInvalidInput,
		}
	}

	h.logger.Info("Performing scroll action via IPC",
		zap.String("action", actionName),
		zap.Int("direction", int(direction)),
		zap.Int("amount", int(amount)),
	)

	targetsSelection := parsed.useSelection

	targetPoint := image.Point{}
	if parsed.useSelection {
		var pointErrResp *ipc.Response

		targetPoint, pointErrResp = h.resolveSelectionPoint()
		if pointErrResp != nil {
			return *pointErrResp
		}
	} else if !parsed.useBare {
		if selectionPoint, ok := h.currentSelectionPoint(); ok {
			targetPoint = selectionPoint
			targetsSelection = true
		}
	}

	if targetsSelection {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: "action service not available",
				Code:    ipc.CodeActionFailed,
			}
		}

		moveErr := h.actionService.MoveCursorToPointAndWait(ctx, targetPoint)
		if moveErr != nil {
			h.logger.Error("Failed to move cursor to scroll target", zap.Error(moveErr))

			return ipc.Response{
				Success: false,
				Message: "failed to perform scroll action: " + moveErr.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}
	}

	scrollErr := h.scrollService.Scroll(ctx, direction, amount)
	if scrollErr != nil {
		h.logger.Error("Scroll action failed", zap.Error(scrollErr),
			zap.String("action", actionName))

		return ipc.Response{
			Success: false,
			Message: "failed to perform scroll action: " + scrollErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: actionName + " performed",
		Code:    ipc.CodeOK,
	}
}

// scrollActionMapping returns the direction, default amount, and validity for a scroll action name.
func scrollActionMapping(name string) (services.ScrollDirection, services.ScrollAmount, bool) {
	switch name {
	case string(action.NameScrollUp):
		return services.ScrollDirectionUp, services.ScrollAmountChar, true
	case string(action.NameScrollDown):
		return services.ScrollDirectionDown, services.ScrollAmountChar, true
	case string(action.NameScrollLeft):
		return services.ScrollDirectionLeft, services.ScrollAmountChar, true
	case string(action.NameScrollRight):
		return services.ScrollDirectionRight, services.ScrollAmountChar, true
	case string(action.NameGoTop):
		return services.ScrollDirectionUp, services.ScrollAmountEnd, true
	case string(action.NameGoBottom):
		return services.ScrollDirectionDown, services.ScrollAmountEnd, true
	case string(action.NamePageUp):
		return services.ScrollDirectionUp, services.ScrollAmountHalfPage, true
	case string(action.NamePageDown):
		return services.ScrollDirectionDown, services.ScrollAmountHalfPage, true
	default:
		return 0, 0, false
	}
}
