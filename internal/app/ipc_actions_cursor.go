package app

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// isMouseButtonActionName reports whether the action name presses, releases,
// toggles, or clicks a mouse button.
func isMouseButtonActionName(actionName string) bool {
	actionType, typeErr := action.ParseType(actionName)
	if typeErr != nil {
		return false
	}

	return actionType.IsMouseButton()
}

// resolveMouseButtonPhase maps a click action carrying --state or --toggle onto
// the action that performs that phase of the click (for example left_click with
// --state down becomes mouse_down). Actions carrying neither flag are returned
// unchanged; every other action rejects the flags.
func resolveMouseButtonPhase(
	actionName string,
	parsed parsedActionArgs,
) (string, *ipc.Response) {
	if !parsed.hasState && !parsed.useToggle {
		return actionName, nil
	}

	if parsed.hasState && parsed.useToggle {
		return "", &ipc.Response{
			Success: false,
			Message: "--state and --toggle cannot be used together",
			Code:    ipc.CodeInvalidInput,
		}
	}

	actionType, typeErr := action.ParseType(actionName)
	if typeErr != nil {
		return "", &ipc.Response{
			Success: false,
			Message: msgStateOnlyOnClicks,
			Code:    ipc.CodeInvalidInput,
		}
	}

	button, phase, isMouseButton := actionType.MouseButtonPhase()
	if !isMouseButton || phase != action.PhaseClick {
		return "", &ipc.Response{
			Success: false,
			Message: msgStateOnlyOnClicks,
			Code:    ipc.CodeInvalidInput,
		}
	}

	targetPhase := action.PhaseToggle

	if parsed.hasState {
		parsedPhase, phaseErr := action.ParsePhase(parsed.stateStr)
		if phaseErr != nil {
			return "", &ipc.Response{
				Success: false,
				Message: phaseErr.Error(),
				Code:    ipc.CodeInvalidInput,
			}
		}

		targetPhase = parsedPhase
	}

	resolved, ok := action.MouseButtonName(button, targetPhase)
	if !ok {
		return "", &ipc.Response{
			Success: false,
			Message: msgStateOnlyOnClicks,
			Code:    ipc.CodeInvalidInput,
		}
	}

	return string(resolved), nil
}

func (h *IPCControllerActions) handleMoveMouseAction(
	ctx context.Context,
	parsed parsedActionArgs,
) (*ipc.Response, error) {
	if parsed.hasX && parsed.hasY {
		if h.actionService == nil {
			return &ipc.Response{
				Success: false,
				Message: msgActionServiceNotAvailable,
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
			Message: msgActionServiceNotAvailable,
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
			Message: msgActionServiceNotAvailable,
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
			Message: msgActionServiceNotAvailable,
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
		Message: "move_mouse requires --x and --y flags, --center, --window, active selection, or --bare",
		Code:    ipc.CodeInvalidInput,
	}
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
			Message: msgSelectionRequiresActiveSelection,
			Code:    ipc.CodeInvalidInput,
		}
	}

	targetPoint, ok := h.modesHandler.CurrentSelectionPoint()
	if !ok {
		return image.Point{}, &ipc.Response{
			Success: false,
			Message: msgSelectionRequiresActiveSelection,
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

func (h *IPCControllerActions) handleCursorVisibilityAction(
	parsed parsedActionArgs,
	hide bool,
) ipc.Response {
	if hasUnsupportedFlags(parsed) {
		actionName := "show_cursor"
		if hide {
			actionName = "hide_cursor"
		}

		return ipc.Response{
			Success: false,
			Message: actionName + " does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	if !h.modesHandler.CursorVisibilitySupported() {
		actionName := "show_cursor"
		if hide {
			actionName = "hide_cursor"
		}

		return ipc.Response{
			Success: false,
			Message: actionName + " is not supported on this platform",
			Code:    ipc.CodeNotSupported,
		}
	}

	if hide {
		h.modesHandler.HideSystemCursor()

		return ipc.Response{Success: true, Message: "system cursor hidden", Code: ipc.CodeOK}
	}

	h.modesHandler.ShowSystemCursor()

	return ipc.Response{Success: true, Message: "system cursor shown", Code: ipc.CodeOK}
}

// handleMoveMonitorAction moves the cursor (and any active mode overlay)
// to a specific monitor by name, or cycles to the next/previous monitor.
// Without --name, cycles to the next monitor (use --previous to go backwards).
func (h *IPCControllerActions) handleMoveMonitorAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.modifierStr != "" ||
		parsed.useSelection || parsed.useBare {
		return ipc.Response{
			Success: false,
			Message: msgMoveMonitorDoesNotSupportTheseFlags,
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasMonitorName {
		if parsed.usePrevious {
			return ipc.Response{
				Success: false,
				Message: "--previous cannot be used with --name",
				Code:    ipc.CodeInvalidInput,
			}
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	if parsed.hasMonitorName {
		h.logger.Debug("Moving to monitor by name via IPC",
			zap.String("monitor", parsed.monitorName),
		)

		err := h.modesHandler.MoveMonitorByName(ctx, parsed.monitorName)
		if err != nil {
			h.logger.Error("Failed to move to monitor by name", zap.Error(err))

			return ipc.Response{
				Success: false,
				Message: "failed to move to monitor: " + err.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}

		return ipc.Response{
			Success: true,
			Message: "move_monitor performed",
			Code:    ipc.CodeOK,
		}
	}

	direction := modes.MonitorDirectionNext
	if parsed.usePrevious {
		direction = modes.MonitorDirectionPrevious
	}

	h.logger.Debug("Cycling monitor via IPC",
		zap.Bool("previous", parsed.usePrevious),
	)

	err := h.modesHandler.MoveMonitor(ctx, direction)
	if err != nil {
		h.logger.Error("Failed to cycle monitor", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to cycle monitor: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "move_monitor performed",
		Code:    ipc.CodeOK,
	}
}
