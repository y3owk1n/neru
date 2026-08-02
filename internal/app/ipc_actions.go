package app

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// IPCControllerActions handles action-related IPC commands.
type IPCControllerActions struct {
	actionService *services.ActionService
	scrollService *services.ScrollService
	modesHandler  *modes.Handler
	appState      *state.AppState
	keyFeed       ports.KeyFeedPort
	logger        *zap.Logger

	// cursorSlots holds the positions save_cursor_pos captured, by slot name.
	// It is shared with the info handler, which reports the occupied slots.
	cursorSlots *state.CursorSlots
}

const (
	actionCmd     = "action"
	flagCenter    = "--center"
	flagWindow    = "--window"
	flagSelection = "--selection"
	flagPrevious  = "--previous"
	flagName      = "--name"
	flagBare      = "--bare"
	flagBail      = "--bail"
	flagState     = "--state"
	flagToggle    = "--toggle"
	flagDirection = "--direction"
	flagCount     = "--count"
	flagSlot      = "--slot"
	flagModifier  = "--modifier"
	flagX         = "--x"
	flagY         = "--y"
	flagDX        = "--dx"
	flagDY        = "--dy"
	flagSteps     = "--steps"
	flagBackward  = "--backward"

	// slotDataKey names the slot a cursor action acted on, in its response data.
	slotDataKey = "slot"

	msgActionServiceNotAvailable            = "action service not available"
	msgModesHandlerNotAvailable             = "modes handler not available"
	msgSelectionRequiresActiveSelection     = "--selection requires an active mode selection"
	msgSelectionAndBareCannotBeUsedTogether = "--selection and --bare cannot be used together"
	msgStateOnlyOnClicks                    = "--state and --toggle are only supported with " +
		"left_click, right_click, and middle_click"

	// interActionDelay is the pause between actions in a comma-separated chain.
	// This gives the OS time to process each click before the next one arrives,
	// enabling the native click-counting layer to detect multi-click sequences.
	interActionDelay = 75 * time.Millisecond
)

// NewIPCControllerActions creates a new action command handler.
func NewIPCControllerActions(
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	modesHandler *modes.Handler,
	appState *state.AppState,
	keyFeed ports.KeyFeedPort,
	cursorSlots *state.CursorSlots,
	logger *zap.Logger,
) *IPCControllerActions {
	// A nil store would make every save panic rather than degrade, and the
	// slots have no dependencies to be missing, so build one instead.
	if cursorSlots == nil {
		cursorSlots = state.NewCursorSlots()
	}

	return &IPCControllerActions{
		actionService: actionService,
		scrollService: scrollService,
		modesHandler:  modesHandler,
		appState:      appState,
		keyFeed:       keyFeed,
		cursorSlots:   cursorSlots,
		logger:        logger,
	}
}

// RegisterHandlers registers action command handlers.
func (h *IPCControllerActions) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[actionCmd] = h.handleAction
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

	if action.IsFeedAction(actionName) {
		return h.handleFeedAction(ctx, cmd.Args[1:])
	}

	if action.Name(actionName) == action.NameSleep {
		return h.handleSleepAction(ctx, cmd.Args[1:])
	}

	parsed, parseErr := parseActionArgs(cmd.Args[1:])
	if parseErr {
		return ipc.Response{
			Success: false,
			Message: "invalid or missing flag value",
			Code:    ipc.CodeInvalidInput,
		}
	}

	// Every action declares the flags it accepts (see actionFlagSupport), so
	// this one check covers all of them and each handler only has to validate
	// combinations of the flags it does accept.
	flagSupportResp := rejectUnsupportedFlags(actionName, parsed)
	if flagSupportResp != nil {
		return *flagSupportResp
	}

	// A click action carrying --state or --toggle is a request for one half of
	// that click, so it is dispatched as the matching press/release/toggle
	// action from here on. The requested name is kept for the reply.
	requestedActionName := actionName

	actionName, phaseErrResp := resolveMouseButtonPhase(actionName, parsed)
	if phaseErrResp != nil {
		return *phaseErrResp
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

	if action.IsMoveCellAction(actionName) {
		return h.handleMoveCellAction(parsed)
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

	if action.IsMoveMonitorAction(actionName) {
		return h.handleMoveMonitorAction(ctx, parsed)
	}

	if action.IsCycleHintAction(actionName) {
		return h.handleCycleHintAction(ctx, parsed)
	}

	if action.IsSearchHintsAction(actionName) {
		return h.handleSearchHintsAction()
	}

	if action.IsHideCursorAction(actionName) || action.IsShowCursorAction(actionName) {
		return h.handleCursorVisibilityAction(action.IsHideCursorAction(actionName))
	}

	// Handle comma-separated action chains (e.g., "left_click,left_click")
	// which produce multi-click sequences via the native click-counting layer.
	// Only mouse button actions are allowed in chains.
	if strings.Contains(actionName, ",") {
		return h.handleActionChain(ctx, cmd, parsed)
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

	flagErrResp := validateActionFlags(actionName, parsed)
	if flagErrResp != nil {
		return *flagErrResp
	}

	// Merge sticky modifiers AFTER the explicit --modifier validation above,
	// so that active sticky modifiers don't cause false rejection of
	// non-click actions like move_mouse or move_mouse_relative.
	if h.modesHandler != nil {
		stickyMods := h.modesHandler.StickyModifiers()
		modifiers |= stickyMods
	}

	if isMoveMouse && parsed.hasCenter {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: msgActionServiceNotAvailable,
				Code:    ipc.CodeActionFailed,
			}
		}

		offsetX, offsetY := parsed.xVal, parsed.yVal

		h.logger.Debug("Moving mouse to center via IPC",
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

	if isMoveMouse && parsed.hasWindow {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: msgActionServiceNotAvailable,
				Code:    ipc.CodeActionFailed,
			}
		}

		offsetX, offsetY := parsed.xVal, parsed.yVal

		h.logger.Debug("Moving mouse to window center via IPC",
			zap.Int("offsetX", offsetX),
			zap.Int("offsetY", offsetY),
		)

		err := h.actionService.MoveMouseToCenterOfWindow(ctx, offsetX, offsetY)
		if err != nil {
			h.logger.Error("Failed to move mouse to window center", zap.Error(err))

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
				Message: msgActionServiceNotAvailable,
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

		h.logger.Debug("Moving mouse relative via IPC",
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

	h.logger.Debug("Performing action via IPC",
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
		Message: requestedActionName + " performed",
		Code:    ipc.CodeOK,
	}
}
