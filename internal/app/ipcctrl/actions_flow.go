package ipcctrl

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
)

const modeExitPollInterval = 10 * time.Millisecond

// modeExitTimeout is the maximum time wait_for_mode_exit will block before
// giving up. This prevents goroutine leaks when the mode never exits (e.g.
// the user abandons the workflow). 5 minutes is generous for any interactive
// mode session.
const modeExitTimeout = 5 * time.Minute

func (h *ActionsHandler) handleSleepAction(
	ctx context.Context,
	args []string,
) ipc.Response {
	durationStr := ""
	for idx := 0; idx < len(args); idx++ {
		arg := strings.TrimSpace(args[idx])
		if after, ok := strings.CutPrefix(arg, "--duration="); ok {
			durationStr = after
		} else if arg == "--duration" {
			val, newIdx, ok := extractStringFlag(args, idx, "--duration")
			idx = newIdx

			if ok && val != "" {
				durationStr = val
			}
		} else if !strings.HasPrefix(arg, "--") && arg != "" && durationStr == "" {
			durationStr = arg
		}
	}

	duration, err := parseSleepDuration(durationStr)
	if err != nil {
		return ipc.Response{
			Success: false,
			Message: err.Error(),
			Code:    ipc.CodeInvalidInput,
		}
	}

	h.logger.Debug("sleep action sleeping", zap.Duration("duration", duration))

	// Wait on a timer rather than time.Sleep so that a long pause inside an
	// action sequence is released when the daemon shuts down instead of
	// holding its goroutine to the end of the duration.
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ipc.Response{
			Success: false,
			Message: "sleep canceled: " + ctx.Err().Error(),
			Code:    ipc.CodeActionFailed,
		}
	case <-timer.C:
	}

	return ipc.Response{
		Success: true,
		Message: "sleep performed",
		Code:    ipc.CodeOK,
	}
}

func parseSleepDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, derrors.New(
			derrors.CodeInvalidInput,
			"sleep requires a duration (e.g., 0.2s, 200ms)",
		)
	}

	duration, err := time.ParseDuration(durationStr)
	if err == nil {
		if duration <= 0 {
			return 0, derrors.Newf(
				derrors.CodeInvalidInput,
				"sleep duration must be positive, got %s",
				durationStr,
			)
		}

		return duration, nil
	}

	secs, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"invalid sleep duration %q: expected a number (seconds) or a duration string such as 200ms or 1.5s",
			durationStr,
		)
	}

	if secs <= 0 {
		return 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"sleep duration must be positive, got %s",
			durationStr,
		)
	}

	return time.Duration(secs * float64(time.Second)), nil
}

func (h *ActionsHandler) handleResetAction() ipc.Response {
	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modesHandler.ResetCurrentMode()

	return ipc.Response{Success: true, Message: "mode reset", Code: ipc.CodeOK}
}

func (h *ActionsHandler) handleWaitForModeExitAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
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

	// Always consume the exit reason to prevent stale values from
	// leaking into a subsequent wait_for_mode_exit --bail call.
	reason := h.appState.ConsumeModeExitReason()

	if parsed.useBail && reason != state.ModeExitReasonCompleted {
		return ipc.Response{
			Success: false,
			Message: "mode exited without selection",
			Code:    ipc.CodeChainBail,
		}
	}

	return ipc.Response{Success: true, Message: "mode exited", Code: ipc.CodeOK}
}

// handleActionChain executes a comma-separated chain of mouse button actions
// at one target point ("left_click,left_click" is a double-click). The native
// click-counting layer turns sequential clicks into multi-click events.
func (h *ActionsHandler) handleActionChain(
	ctx context.Context,
	cmd ipc.Command,
	parsed parsedActionArgs,
) ipc.Response {
	actionName := cmd.Args[0]
	actions := strings.Split(actionName, ",")

	if resp := validateActionChain(actions); resp != nil {
		return *resp
	}

	modifiers, modErr := action.ParseModifiers(parsed.modifierStr)
	if modErr != nil {
		return refuseAction(modErr.Error())
	}

	if h.modesHandler != nil {
		modifiers |= h.modesHandler.StickyModifiers()
	}

	if parsed.useSelection && parsed.useBare {
		return refuseAction(msgSelectionAndBareCannotBeUsedTogether)
	}

	if h.actionService == nil {
		return failAction(msgActionServiceNotAvailable)
	}

	// One target point for the whole chain.
	targetPoint, pointErrResp := h.resolveMouseActionPoint(ctx, parsed)
	if pointErrResp != nil {
		return *pointErrResp
	}

	if resp := h.moveToSelectionTarget(ctx, parsed, targetPoint); resp != nil {
		return *resp
	}

	return h.performActionChain(ctx, actionName, actions, targetPoint, modifiers)
}

// validateActionChain refuses a chain containing anything but known mouse
// button actions.
func validateActionChain(actions []string) *ipc.Response {
	for actionIdx, a := range actions {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			resp := refuseAction(fmt.Sprintf(
				"invalid action at position %d: empty action in comma-separated list",
				actionIdx,
			))

			return &resp
		}

		if !action.IsKnownName(action.Name(trimmed)) {
			resp := refuseAction(fmt.Sprintf(
				"invalid action: %s. Supported actions: %s",
				trimmed,
				action.SupportedNamesString(),
			))

			return &resp
		}

		if action.IsScrollSubAction(trimmed) {
			resp := refuseAction(fmt.Sprintf(
				"scroll sub-action %q cannot be used in an action chain",
				trimmed,
			))

			return &resp
		}

		actType, err := action.Name(trimmed).ToType()
		if err != nil || !actType.IsMouseButton() {
			resp := refuseAction(fmt.Sprintf(
				"%q cannot be used in an action chain; only mouse button actions are allowed",
				trimmed,
			))

			return &resp
		}
	}

	return nil
}

// moveToSelectionTarget moves the cursor to the target when it is the mode
// selection, and waits for it to land so the clicks hit the right spot.
func (h *ActionsHandler) moveToSelectionTarget(
	ctx context.Context,
	parsed parsedActionArgs,
	targetPoint image.Point,
) *ipc.Response {
	targetsSelection := parsed.useSelection
	if !targetsSelection && !parsed.useBare {
		if selectionPoint, ok := h.currentSelectionPoint(); ok &&
			targetPoint == selectionPoint {
			targetsSelection = true
		}
	}

	if !targetsSelection {
		return nil
	}

	moveErr := h.actionService.MoveCursorToPointAndWait(ctx, targetPoint)
	if moveErr != nil {
		h.logger.Error("Failed to move cursor to mode selection", zap.Error(moveErr))

		resp := failAction("failed to perform action: " + moveErr.Error())

		return &resp
	}

	return nil
}

// performActionChain fires each action at the same point with the same
// modifiers.
func (h *ActionsHandler) performActionChain(
	ctx context.Context,
	actionName string,
	actions []string,
	targetPoint image.Point,
	modifiers action.Modifiers,
) ipc.Response {
	for actionIdx, a := range actions {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			continue
		}

		// Give the OS time to process each click, so its click-counting
		// (a ~500ms window) produces real double- and triple-clicks.
		if actionIdx > 0 {
			time.Sleep(interActionDelay)
		}

		h.logger.Debug("Executing action in chain",
			zap.String("action", trimmed),
			zap.Int("x", targetPoint.X),
			zap.Int("y", targetPoint.Y),
		)

		performErr := h.actionService.PerformActionAtPoint(
			ctx,
			trimmed,
			targetPoint,
			modifiers,
		)
		if performErr != nil {
			h.logger.Error("Failed to perform action in chain",
				zap.Error(performErr),
				zap.String("action", trimmed))

			return failAction("failed to perform action in chain: " + performErr.Error())
		}
	}

	return ipc.Response{Success: true, Message: actionName + " performed", Code: ipc.CodeOK}
}
