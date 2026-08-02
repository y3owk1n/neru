package app

import (
	"context"
	"fmt"
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

func (h *IPCControllerActions) handleSleepAction(
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

func (h *IPCControllerActions) handleResetAction() ipc.Response {
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

func (h *IPCControllerActions) handleWaitForModeExitAction(
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
// at the same target point (e.g., "left_click,left_click" for a double-click).
// The native click-counting layer automatically converts sequential clicks into
// multi-click events (clickCount=2, clickCount=3...).
func (h *IPCControllerActions) handleActionChain(
	ctx context.Context,
	cmd ipc.Command,
	parsed parsedActionArgs,
) ipc.Response {
	actionName := cmd.Args[0]

	// Split comma-separated actions
	actions := strings.Split(actionName, ",")

	// Validate each action in the chain
	for actionIdx, a := range actions {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf(
					"invalid action at position %d: empty action in comma-separated list",
					actionIdx,
				),
				Code: ipc.CodeInvalidInput,
			}
		}

		if !action.IsKnownName(action.Name(trimmed)) {
			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf(
					"invalid action: %s. Supported actions: %s",
					trimmed,
					action.SupportedNamesString(),
				),
				Code: ipc.CodeInvalidInput,
			}
		}

		// Chain only supports mouse button actions (left_click, right_click, etc.)
		// for multi-click sequences.
		if action.IsScrollSubAction(trimmed) {
			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf(
					"scroll sub-action %q cannot be used in an action chain",
					trimmed,
				),
				Code: ipc.CodeInvalidInput,
			}
		}

		actType, err := action.Name(trimmed).ToType()
		if err != nil || !actType.IsMouseButton() {
			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf(
					"%q cannot be used in an action chain; only mouse button actions are allowed",
					trimmed,
				),
				Code: ipc.CodeInvalidInput,
			}
		}
	}

	// Parse modifiers
	modifiers, modErr := action.ParseModifiers(parsed.modifierStr)
	if modErr != nil {
		return ipc.Response{
			Success: false,
			Message: modErr.Error(),
			Code:    ipc.CodeInvalidInput,
		}
	}

	// Merge sticky modifiers
	if h.modesHandler != nil {
		stickyMods := h.modesHandler.StickyModifiers()
		modifiers |= stickyMods
	}

	if parsed.useSelection && parsed.useBare {
		return ipc.Response{
			Success: false,
			Message: msgSelectionAndBareCannotBeUsedTogether,
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.actionService == nil {
		return ipc.Response{
			Success: false,
			Message: msgActionServiceNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	// Resolve target point once for all actions in the chain
	targetPoint, pointErrResp := h.resolveMouseActionPoint(ctx, parsed)
	if pointErrResp != nil {
		return *pointErrResp
	}

	// Move cursor to selection target if needed
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

			return ipc.Response{
				Success: false,
				Message: "failed to perform action: " + moveErr.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}
	}

	// Execute each action in the chain at the same point with the same modifiers.
	for actionIdx, a := range actions {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			continue
		}

		// Add a delay between actions so the OS has time to process each
		// click before the next one fires. This ensures the native
		// click-counting (which tracks clickCount within a ~500ms window)
		// correctly produces double-click and triple-click events.
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

			return ipc.Response{
				Success: false,
				Message: "failed to perform action in chain: " + performErr.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}
	}

	return ipc.Response{Success: true, Message: actionName + " performed", Code: ipc.CodeOK}
}
