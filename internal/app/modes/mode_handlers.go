package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// executeActionAtPoint executes a pending action at the given point and exits the mode.
func (h *Handler) executeActionAtPoint(action *string, point image.Point) {
	if action == nil {
		h.logger.Warn("executeActionAtPoint called with nil action")

		return
	}

	h.logger.Info("Executing pending action", zap.String("action", *action))

	ctx := context.Background()

	performActionErr := h.actionService.PerformAction(ctx, *action, point)
	if performActionErr != nil {
		h.logger.Error("Failed to perform pending action", zap.Error(performActionErr))
	}

	h.ExitMode()
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
		h.executeActionAtPoint(pendingAction, point)

		return
	}

	// No pending action - re-activate mode if requested
	if shouldReActivate && reActivateFunc != nil {
		h.logger.Info("Re-activating mode after cursor movement")
		reActivateFunc()
	}
}

// handleHintsModeKey handles key processing for hints mode.
func (h *Handler) handleHintsModeKey(key string) {
	ctx := context.Background()

	if h.actionService.IsDirectActionKey(key) {
		h.actionService.HandleDirectActionKey(ctx, key)

		return
	}

	// Route hint-specific keys via domain hints router
	if h.hints.Context.Router() == nil {
		h.logger.Error("Hints router is nil")
		h.ExitMode()

		return
	}

	hintKeyResult := h.hints.Context.Router().RouteKey(key)
	if hintKeyResult.Exit() {
		h.ExitMode()

		return
	}

	// Hint input processed by router; if exact match, perform action
	if hintKeyResult.ExactHint() != nil {
		hint := hintKeyResult.ExactHint()
		center := hint.Element().Center()

		h.logger.Info("Found element", zap.String("label", hint.Label()))

		h.moveCursorAndHandleAction(
			center,
			h.hints.Context.PendingAction(),
			true,
			func() { h.activateHintModeInternal(false, nil) },
		)
	}
}

// handleGridModeKey handles key processing for grid mode.
func (h *Handler) handleGridModeKey(key string) {
	ctx := context.Background()

	if h.actionService.IsDirectActionKey(key) {
		h.actionService.HandleDirectActionKey(ctx, key)

		return
	}

	if h.grid.Router == nil {
		h.logger.Error("Grid router is nil")
		h.ExitMode()

		return
	}

	gridKeyResult := h.grid.Router.RouteKey(key)
	if gridKeyResult.Exit() {
		h.ExitMode()

		return
	}

	if gridKeyResult.Complete() {
		targetPoint := gridKeyResult.TargetPoint()

		// Convert from window-local coordinates to absolute screen coordinates using helper
		screenBounds := bridge.ActiveScreenBounds()
		absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, screenBounds)

		h.logger.Info(
			"Grid move mouse",
			zap.Int("x", absolutePoint.X),
			zap.Int("y", absolutePoint.Y),
		)

		h.moveCursorAndHandleAction(
			absolutePoint,
			h.grid.Context.PendingAction(),
			false, // Grid mode doesn't re-activate after cursor movement
			nil,
		)
	}
}
