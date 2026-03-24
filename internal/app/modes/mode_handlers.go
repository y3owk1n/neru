package modes

import (
	"context"
	"image"
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
)

// executeActionAtPoint executes a pending action at the given point and exits the mode.
// When repeat is true and reActivateFunc is provided, the mode is re-activated
// instead of exiting after performing the action.
func (h *Handler) executeActionAtPoint(
	action *string,
	point image.Point,
	repeat bool,
	reActivateFunc func(),
) {
	if action == nil {
		h.logger.Warn("executeActionAtPoint called with nil action")

		return
	}

	h.logger.Info("Executing pending action",
		zap.String("action", *action),
		zap.String("modifiers", h.stickyModifiers().String()),
		zap.Bool("repeat", repeat))

	ctx := context.Background()

	performActionErr := h.actionService.PerformActionAtPoint(
		ctx,
		*action,
		point,
		h.stickyModifiers(),
	)
	if performActionErr != nil {
		h.logger.Error("Failed to perform pending action", zap.Error(performActionErr))
	}

	// Signal that a click was just performed so handleCursorRestoration
	// can insert a settling delay before moving the cursor.
	// Skip move-mouse actions — they don't produce clicks that need settling.
	if performActionErr == nil &&
		*action != "move_mouse" &&
		*action != "move_mouse_relative" {
		h.cursorState.MarkActionPerformed()
	}

	if repeat && reActivateFunc != nil {
		// Wait for the target app to finish processing the click before
		// re-activating (which may move the cursor for grid/recursive-grid).
		// This mirrors the settle delay in handleCursorRestoration and
		// prevents slow apps (Electron, web views) from missing clicks.
		if h.cursorState.WasActionPerformed() {
			time.Sleep(postActionSettleDelay)
		}

		h.logger.Info("Re-activating mode after action (--repeat)")
		reActivateFunc()

		return
	}

	h.exitModeLocked()
}

// moveCursorAndHandleAction moves the cursor to a point and executes any pending action.
func (h *Handler) moveCursorAndHandleAction(
	point image.Point,
	pendingAction *string,
	shouldReActivate bool,
	reActivateFunc func(),
) {
	ctx := context.Background()

	moveCursorErr := h.actionService.MoveCursorToPoint(ctx, point)
	if moveCursorErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
	}

	if pendingAction != nil {
		h.executeActionAtPoint(pendingAction, point, shouldReActivate, reActivateFunc)

		return
	}

	// No pending action - re-activate mode if requested
	if shouldReActivate && reActivateFunc != nil {
		h.logger.Info("Re-activating mode after cursor movement")
		reActivateFunc()
	}
}

// shouldAutoExit checks if the given action name is in the auto-exit list.
func (h *Handler) shouldAutoExit(autoExitActions []string, actionName string) bool {
	return len(autoExitActions) > 0 && slices.Contains(autoExitActions, actionName)
}

// handleHintsModeKey handles key processing for hints mode.
func (h *Handler) handleHintsModeKey(key string) {
	ctx := context.Background()

	actionName, wasHandled, err := h.actionService.HandleDirectActionKey(
		ctx,
		key,
		h.stickyModifiers(),
	)
	if wasHandled {
		if err != nil {
			h.logger.Error("Failed to handle direct action key", zap.Error(err))

			return
		}

		if h.shouldAutoExit(h.config.Hints.AutoExitActions, actionName) {
			if !h.actionService.IsMoveMouseKey(key) {
				h.cursorState.MarkActionPerformed()
			}

			h.exitModeLocked()

			return
		}

		// Only refresh hints after non-move-mouse actions
		// Move mouse actions should keep the overlay active
		if !h.actionService.IsMoveMouseKey(key) {
			// Capture repeat state before refresh so we can restore it on the
			// fresh context. activateHintModeInternal resets the context,
			// which would lose the --repeat and pendingAction flags.
			savedPendingAction := h.hints.Context.PendingAction()
			savedRepeat := h.hints.Context.Repeat()

			// restoreRepeatState restores the repeat and pendingAction flags
			// on the fresh hints context after a refresh re-activation.
			restoreRepeatState := func() {
				if savedRepeat && h.appState.CurrentMode() == domain.ModeHints &&
					h.hints != nil && h.hints.Context != nil {
					h.hints.Context.SetPendingAction(savedPendingAction)
					h.hints.Context.SetRepeat(true)
				}
			}

			bundleID, err := h.actionService.FocusedAppBundleID(ctx)
			if err != nil {
				h.logger.Warn(
					"Failed to get focused app bundle ID, using global delay",
					zap.Error(err),
				)

				bundleID = ""
			}

			delay := h.config.MouseActionRefreshDelayForApp(bundleID)

			if delay == 0 {
				h.activateHintModeInternal(false, nil)
				restoreRepeatState()
			} else {
				if h.refreshHintsTimer != nil {
					h.refreshHintsTimer.Stop()
				}

				var _timer *time.Timer

				timerSession := h.modeSession

				_timer = time.AfterFunc(
					time.Duration(delay)*time.Millisecond,
					func() {
						// Lock to serialize with HandleKeyPress on the event tap thread
						h.mu.Lock()
						defer h.mu.Unlock()

						// Guard against stale timer: if the user exited hints mode
						// (e.g. pressed escape) while we were waiting for the lock,
						// or if hints was re-entered (new session), do not
						// re-activate.
						if h.modeSession != timerSession ||
							h.appState.CurrentMode() != domain.ModeHints {
							return
						}

						// Clear our own timer reference only if we are still the active one.
						// A newer timer may have replaced us while we were waiting for the lock.
						if h.refreshHintsTimer == _timer {
							h.refreshHintsTimer = nil
						}

						h.activateHintModeInternal(false, nil)
						restoreRepeatState()
					},
				)

				h.refreshHintsTimer = _timer
			}
		}

		return
	}

	// Route hint-specific keys via domain hints router
	if h.hints.Context.Router() == nil {
		h.logger.Warn("Hints router is nil - ignoring key press until hints initialized")

		return
	}

	hintKeyResult := h.hints.Context.Router().RouteKey(key)
	if hintKeyResult.Exit() {
		h.exitModeLocked()

		return
	}

	// Hint input processed by router; if exact match, perform action
	if hintKeyResult.ExactHint() != nil {
		hint := hintKeyResult.ExactHint()
		center := hint.Element().Center()

		h.logger.Info("Found element", zap.String("label", hint.Label()))

		pendingAction := h.hints.Context.PendingAction()
		repeat := h.hints.Context.Repeat()

		h.moveCursorAndHandleAction(
			center,
			pendingAction,
			repeat ||
				pendingAction == nil, // re-activate on repeat, or when no action (existing behavior)
			func() {
				h.activateHintModeInternal(false, nil)
				// Restore repeat and action on the fresh context so subsequent
				// selections continue the repeat cycle.
				// Guard: only restore if re-activation succeeded (mode is still hints).
				if repeat && h.appState.CurrentMode() == domain.ModeHints &&
					h.hints != nil && h.hints.Context != nil {
					h.hints.Context.SetPendingAction(pendingAction)
					h.hints.Context.SetRepeat(true)
				}
			},
		)
	}
}

// handleGridModeKey handles key processing for grid mode.
func (h *Handler) handleGridModeKey(key string) {
	ctx := context.Background()

	actionName, wasHandled, err := h.actionService.HandleDirectActionKey(
		ctx,
		key,
		h.stickyModifiers(),
	)
	if wasHandled {
		if err != nil {
			h.logger.Error("Failed to handle direct action key", zap.Error(err))

			return
		}

		if h.shouldAutoExit(h.config.Grid.AutoExitActions, actionName) {
			if !h.actionService.IsMoveMouseKey(key) {
				h.cursorState.MarkActionPerformed()
			}

			h.exitModeLocked()
		}

		return
	}

	if h.grid.Router == nil {
		h.logger.Warn("Grid router is nil - ignoring key press until grid router initialized")

		return
	}

	gridKeyResult := h.grid.Router.RouteKey(key)
	if gridKeyResult.Exit() {
		h.exitModeLocked()

		return
	}

	if gridKeyResult.Complete() {
		targetPoint := gridKeyResult.TargetPoint()

		// Convert from window-local coordinates to absolute screen coordinates using helper
		absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)

		h.logger.Info(
			"Grid move mouse",
			zap.Int("x", absolutePoint.X),
			zap.Int("y", absolutePoint.Y),
		)

		repeat := h.grid.Context.Repeat()
		pendingAction := h.grid.Context.PendingAction()

		h.moveCursorAndHandleAction(
			absolutePoint,
			pendingAction,
			repeat, // Re-activate grid mode when --repeat is set
			func() { h.activateGridModeWithAction(pendingAction, repeat) },
		)
	}
}
