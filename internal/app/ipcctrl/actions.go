package ipcctrl

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
)

// ActionsHandler handles action-related IPC commands.
type ActionsHandler struct {
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
	// ActionCommand is the IPC command that carries a mouse or key action.
	// It is exported because the hotkey layer recognizes the same word when it
	// decides whether a binding is an action step.
	ActionCommand = action.PrefixAction

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

// NewActionsHandler creates a new action command handler.
func NewActionsHandler(
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	modesHandler *modes.Handler,
	appState *state.AppState,
	keyFeed ports.KeyFeedPort,
	cursorSlots *state.CursorSlots,
	logger *zap.Logger,
) *ActionsHandler {
	// A nil store would make every save panic rather than degrade, and the
	// slots have no dependencies to be missing, so build one instead.
	if cursorSlots == nil {
		cursorSlots = state.NewCursorSlots()
	}

	return &ActionsHandler{
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
func (h *ActionsHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[ActionCommand] = h.handleAction
}

// handleAction routes one `neru action ...` request to the handler for it.
func (h *ActionsHandler) handleAction(ctx context.Context, cmd ipc.Command) ipc.Response {
	if len(cmd.Args) == 0 {
		return refuseAction("action subcommand required (e.g., left_click, right_click)")
	}

	actionName := cmd.Args[0]

	// feed and sleep take their arguments raw rather than as flags, so they are
	// dispatched before anything tries to parse flags out of them.
	if action.IsFeedAction(actionName) {
		return h.handleFeedAction(ctx, cmd.Args[1:])
	}

	if action.Name(actionName) == action.NameSleep {
		return h.handleSleepAction(ctx, cmd.Args[1:])
	}

	parsed, parseFailed := parseActionArgs(cmd.Args[1:])
	if parseFailed {
		return refuseAction("invalid or missing flag value")
	}

	// Every action declares the flags it accepts (see actionFlagSupport), so
	// this one check covers all of them and each handler below only has to
	// validate combinations of the flags it does accept.
	unsupported := rejectUnsupportedFlags(actionName, parsed)
	if unsupported != nil {
		return *unsupported
	}

	// A click action carrying --state or --toggle is a request for one half of
	// that click, so it is dispatched as the matching press/release/toggle
	// action from here on. The requested name is kept for the reply.
	requestedActionName := actionName

	actionName, phaseErrResp := resolveMouseButtonPhase(actionName, parsed)
	if phaseErrResp != nil {
		return *phaseErrResp
	}

	namedResp, handled := h.dispatchByName(ctx, cmd, actionName, parsed)
	if handled {
		return namedResp
	}

	return h.performTargetedAction(ctx, actionName, requestedActionName, parsed)
}

// refuseAction builds the reply for a request that could not be used as given.
func refuseAction(message string) ipc.Response {
	return ipc.Response{
		Success: false,
		Message: message,
		Code:    ipc.CodeInvalidInput,
	}
}

// failAction builds the reply for a request that was understood but could not
// be carried out.
func failAction(message string) ipc.Response {
	return ipc.Response{
		Success: false,
		Message: message,
		Code:    ipc.CodeActionFailed,
	}
}

// dispatchByName routes the actions identified by their name alone. The second
// result reports whether one of them matched; anything else acts on a point and
// is handled after.
func (h *ActionsHandler) dispatchByName(
	ctx context.Context,
	cmd ipc.Command,
	actionName string,
	parsed parsedActionArgs,
) (ipc.Response, bool) {
	switch {
	// Scrolling needs only the scroll service, so it is dispatched ahead of
	// everything that first checks for the action service.
	case action.IsScrollSubAction(actionName):
		return h.handleScrollAction(ctx, actionName, parsed), true

	case action.IsResetAction(actionName):
		return h.handleResetAction(), true

	case action.IsBackspaceAction(actionName):
		return h.handleBackspaceAction(), true

	case action.IsMoveCellAction(actionName):
		return h.handleMoveCellAction(parsed), true

	case action.IsWaitForModeExitAction(actionName):
		return h.handleWaitForModeExitAction(ctx, parsed), true

	case action.IsSaveCursorPosAction(actionName):
		return h.handleSaveCursorPosAction(ctx, parsed), true

	case action.IsRestoreCursorPosAction(actionName):
		return h.handleRestoreCursorPosAction(ctx, parsed), true

	case action.IsMoveMonitorAction(actionName):
		return h.handleMoveMonitorAction(ctx, parsed), true

	case action.IsCycleHintAction(actionName):
		return h.handleCycleHintAction(ctx, parsed), true

	case action.IsSearchHintsAction(actionName):
		return h.handleSearchHintsAction(), true

	case action.IsHideCursorAction(actionName) || action.IsShowCursorAction(actionName):
		return h.handleCursorVisibilityAction(action.IsHideCursorAction(actionName)), true

	// A comma-separated chain produces a multi-click sequence through the
	// native click-counting layer. Only mouse buttons may appear in one.
	case strings.Contains(actionName, ","):
		return h.handleActionChain(ctx, cmd, parsed), true

	default:
		return ipc.Response{}, false
	}
}

// performTargetedAction runs the actions that act on a point: the mouse moves
// and the clicks.
func (h *ActionsHandler) performTargetedAction(
	ctx context.Context,
	actionName string,
	requestedActionName string,
	parsed parsedActionArgs,
) ipc.Response {
	modifiers, modErr := action.ParseModifiers(parsed.modifierStr)
	if modErr != nil {
		return refuseAction(modErr.Error())
	}

	flagErrResp := validateActionFlags(actionName, parsed)
	if flagErrResp != nil {
		return *flagErrResp
	}

	// Sticky modifiers merge only after the explicit --modifier validation
	// above, so that an active sticky modifier cannot make a non-click action
	// such as move_mouse look as though it were given a modifier it does not
	// accept.
	if h.modesHandler != nil {
		modifiers |= h.modesHandler.StickyModifiers()
	}

	moveResp, moved := h.dispatchMouseMove(ctx, actionName, parsed)
	if moved {
		return moveResp
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

		return failAction("failed to perform action: " + err.Error())
	}

	return ipc.Response{
		Success: true,
		Message: requestedActionName + " performed",
		Code:    ipc.CodeOK,
	}
}

// mouseMove is one of the mouse moves that names its own destination rather
// than taking a point.
type mouseMove struct {
	// requires is checked once the action service is known to exist, so that a
	// missing service is still reported before a missing flag.
	requires func() *ipc.Response

	debugMessage string
	failMessage  string
	fields       []zap.Field

	run func() error
}

// dispatchMouseMove runs the mouse moves that name their own destination: the
// screen center, the focused window's center, or an offset from where the
// cursor already is. The second result reports whether one of them applied.
func (h *ActionsHandler) dispatchMouseMove(
	ctx context.Context,
	actionName string,
	parsed parsedActionArgs,
) (ipc.Response, bool) {
	isMoveMouse := actionName == string(action.NameMoveMouse)
	offsetX, offsetY := parsed.xVal, parsed.yVal

	switch {
	case isMoveMouse && parsed.hasCenter:
		return h.runMouseMove(actionName, parsed, mouseMove{
			debugMessage: "Moving mouse to center via IPC",
			failMessage:  "Failed to move mouse to center",
			fields:       []zap.Field{zap.Int("offsetX", offsetX), zap.Int("offsetY", offsetY)},
			run: func() error {
				return h.actionService.MoveMouseToCenter(ctx, offsetX, offsetY)
			},
		}), true

	case isMoveMouse && parsed.hasWindow:
		return h.runMouseMove(actionName, parsed, mouseMove{
			debugMessage: "Moving mouse to window center via IPC",
			failMessage:  "Failed to move mouse to window center",
			fields:       []zap.Field{zap.Int("offsetX", offsetX), zap.Int("offsetY", offsetY)},
			run: func() error {
				return h.actionService.MoveMouseToCenterOfWindow(ctx, offsetX, offsetY)
			},
		}), true

	case actionName == string(action.NameMoveMouseRelative):
		return h.runMouseMove(actionName, parsed, mouseMove{
			requires: func() *ipc.Response {
				if parsed.hasDX && parsed.hasDY {
					return nil
				}

				refusal := refuseAction("move_mouse_relative requires --dx and --dy flags")

				return &refusal
			},
			debugMessage: "Moving mouse relative via IPC",
			failMessage:  "Failed to move mouse relative",
			fields:       []zap.Field{zap.Int("dx", parsed.deltaX), zap.Int("dy", parsed.deltaY)},
			run: func() error {
				return h.actionService.MoveMouseRelative(ctx, parsed.deltaX, parsed.deltaY, true)
			},
		}), true

	default:
		return ipc.Response{}, false
	}
}

// runMouseMove carries out one mouse move. All of them share this shape: the
// action service has to exist, a failure is reported the same way, and a move
// that lands may clear the selection point the mode was holding.
func (h *ActionsHandler) runMouseMove(
	actionName string,
	parsed parsedActionArgs,
	move mouseMove,
) ipc.Response {
	if h.actionService == nil {
		return failAction(msgActionServiceNotAvailable)
	}

	if move.requires != nil {
		refusal := move.requires()
		if refusal != nil {
			return *refusal
		}
	}

	h.logger.Debug(move.debugMessage, move.fields...)

	err := move.run()
	if err != nil {
		h.logger.Error(move.failMessage, zap.Error(err))

		return failAction("failed to perform action: " + err.Error())
	}

	if h.modesHandler != nil && shouldClearSelectionAfterMoveMouse(parsed, false) {
		h.modesHandler.ClearCurrentSelectionPoint()
	}

	return ipc.Response{
		Success: true,
		Message: actionName + " performed",
		Code:    ipc.CodeOK,
	}
}
