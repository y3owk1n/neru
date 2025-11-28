package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	if h.appState.CurrentMode() == domain.ModeIdle {
		if key == "\x1b" || key == "escape" {
			h.logger.Info("Exiting standalone scroll mode")
			h.overlayManager.Clear()
			h.overlayManager.Hide()

			if h.disableEventTap != nil {
				h.disableEventTap()
			}

			h.scroll.Context.SetIsActive(false)
			h.scroll.Context.SetLastKey("")
			// Reset cursor state when exiting scroll mode to ensure proper cursor restoration
			// in subsequent modes
			h.cursorState.Reset()

			return
		}
		// Try to handle scroll keys with generic handler using persistent state.
		// If it's not a scroll key, it will just be ignored.
		lastKey := h.scroll.Context.LastKey()
		h.handleGenericScrollKey(key, &lastKey)
		h.scroll.Context.SetLastKey(lastKey)

		return
	}

	if key == "\t" {
		h.handleTabKey()

		return
	}

	if key == "\x1b" || key == "escape" {
		h.handleEscapeKey()

		return
	}

	h.handleModeSpecificKey(key)
}

// handleTabKey handles the tab key to toggle between overlay mode and action mode.
func (h *Handler) handleTabKey() {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		h.toggleActionModeForHints()
	case domain.ModeGrid:
		h.toggleActionModeForGrid()
	case domain.ModeIdle:
		return
	}
}

// handleEscapeKey handles the escape key to exit action mode or current mode.
func (h *Handler) handleEscapeKey() {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints.Context.InActionMode() {
			h.hints.Context.SetInActionMode(false)
			h.overlayManager.Clear()
			h.overlayManager.Hide()
			h.ExitMode()
			h.logger.Info("Exited hints action mode completely")
			h.overlaySwitch(overlay.ModeIdle)

			return
		}
	case domain.ModeGrid:
		if h.grid.Context.InActionMode() {
			h.grid.Context.SetInActionMode(false)
			h.overlayManager.Clear()
			h.overlayManager.Hide()
			h.ExitMode()
			h.logger.Info("Exited grid action mode completely")
			h.overlaySwitch(overlay.ModeIdle)

			return
		}
	case domain.ModeIdle:
		return
	}

	h.ExitMode()
	h.SetModeIdle()
}

// toggleActionModeForHints toggles between overlay and action mode for hints.
func (h *Handler) toggleActionModeForHints() {
	// Skip tab handling if pending action is set
	if h.hints.Context.PendingAction() != nil {
		h.logger.Debug("Tab key disabled when action is pending")

		return
	}

	if h.hints.Context.InActionMode() {
		h.hints.Context.SetInActionMode(false)

		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}
		// Re-activate hint mode while preserving action mode state
		h.activateHintModeInternal(true, nil)
		h.logger.Info("Switched back to hints overlay mode")
		h.overlaySwitch(overlay.ModeHints)
	} else {
		h.hints.Context.SetInActionMode(true)
		h.overlayManager.Clear()
		h.overlayManager.Hide()
		h.drawActionHighlight()
		h.overlayManager.Show()
		h.logger.Info("Switched to hints action mode")
		h.overlaySwitch(overlay.ModeAction)
	}
}

// toggleActionModeForGrid toggles between overlay and action mode for grid.
func (h *Handler) toggleActionModeForGrid() {
	// Skip tab handling if pending action is set
	if h.grid.Context.PendingAction() != nil {
		h.logger.Debug("Tab key disabled when action is pending")

		return
	}

	if h.grid.Context.InActionMode() {
		h.grid.Context.SetInActionMode(false)
		h.overlayManager.Clear()
		h.overlayManager.Hide()

		// Re-activate grid mode (similar to hints mode pattern)
		h.activateGridModeWithAction(nil)

		h.logger.Info("Switched back to grid overlay mode")
		h.overlaySwitch(overlay.ModeGrid)
	} else {
		h.grid.Context.SetInActionMode(true)
		h.overlayManager.Clear()
		h.overlayManager.Hide()
		h.drawActionHighlight()
		h.overlayManager.Show()
		h.logger.Info("Switched to grid action mode")
		h.overlaySwitch(overlay.ModeAction)
	}
}

// handleHintsModeKey handles key processing for hints mode.
func (h *Handler) handleHintsModeKey(key string) {
	if h.hints.Context.InActionMode() {
		h.handleHintsActionKey(key)
		// After handling the action, we stay in action mode.
		// The user can press Tab to go back to overlay mode or perform more actions.
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
		// Use the domain element's center point
		center := hint.Element().Center()

		h.logger.Info("Found element", zap.String("label", hint.Label()))

		ctx := context.Background()

		moveCursorErr := h.actionService.MoveCursorToPoint(ctx, center)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}

		pendingAction := h.hints.Context.PendingAction()
		if pendingAction != nil {
			h.logger.Info("Executing pending action", zap.String("action", *pendingAction))
			// Use ActionService
			ctx := context.Background()

			performActionErr := h.actionService.PerformAction(ctx, *pendingAction, center)
			if performActionErr != nil {
				h.logger.Error("Failed to perform pending action", zap.Error(performActionErr))
			}
			// Exit mode after executing action
			h.ExitMode()

			return
		}

		// No pending action - re-activate hints mode to show hints again
		h.logger.Info("Re-activating hints mode after cursor movement")
		h.activateHintModeInternal(false, nil)

		return
	}
}

// handleGridModeKey handles key processing for grid mode.
func (h *Handler) handleGridModeKey(key string) {
	if h.grid.Context.InActionMode() {
		h.handleGridActionKey(key)
		// After handling the action, we stay in action mode.
		// The user can press Tab to go back to overlay mode or perform more actions.
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

		ctx := context.Background()

		moveCursorErr := h.actionService.MoveCursorToPoint(ctx, absolutePoint)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}

		pendingAction := h.grid.Context.PendingAction()
		if pendingAction != nil {
			h.logger.Info("Executing pending action", zap.String("action", *pendingAction))
			// Use ActionService
			ctx := context.Background()

			performActionErr := h.actionService.PerformAction(
				ctx,
				*pendingAction,
				absolutePoint,
			)
			if performActionErr != nil {
				h.logger.Error("Failed to perform pending action", zap.Error(performActionErr))
			}
			// Exit mode after executing action
			h.ExitMode()

			return
		}

		// No need to exit grid mode, just let it going

		return
	}
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *Handler) handleModeSpecificKey(key string) {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		h.handleHintsModeKey(key)
	case domain.ModeGrid:
		h.handleGridModeKey(key)
	case domain.ModeIdle:
		return
	}
}

// ExitMode exits the current mode.
func (h *Handler) ExitMode() {
	if h.appState.CurrentMode() == domain.ModeIdle {
		return
	}

	h.logger.Info("Exiting current mode", zap.String("mode", h.CurrModeString()))

	h.performModeSpecificCleanup()
	h.performCommonCleanup()
	h.handleCursorRestoration()
}

// performModeSpecificCleanup handles mode-specific cleanup logic.
func (h *Handler) performModeSpecificCleanup() {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		h.cleanupHintsMode()
	case domain.ModeGrid:
		h.cleanupGridMode()
	case domain.ModeIdle:
		// No specific cleanup needed for idle mode
		return
	default:
		h.cleanupDefaultMode()
	}
}

// clearAndHideOverlay clears and hides the overlay manager.
func (h *Handler) clearAndHideOverlay() {
	h.overlayManager.Clear()
	h.overlayManager.Hide()
}

// cleanupHintsMode handles cleanup for hints mode.
func (h *Handler) cleanupHintsMode() {
	h.hints.Context.SetInActionMode(false)
	h.hints.Context.Reset()

	h.clearAndHideOverlay()
}

// cleanupDefaultMode handles cleanup for default/unknown modes.
func (h *Handler) cleanupDefaultMode() {
	// No domain-specific cleanup for other modes yet.
	// But still clear and hide action overlay.
	if overlay.Get() != nil {
		overlay.Get().Clear()
		overlay.Get().Hide()
	}
}

// cleanupGridMode handles cleanup for grid mode.
func (h *Handler) cleanupGridMode() {
	h.grid.Context.SetInActionMode(false)

	if h.grid.Manager != nil {
		h.grid.Manager.Reset()
	}

	h.clearAndHideOverlay()
}

// performCommonCleanup handles common cleanup logic for all modes.
func (h *Handler) performCommonCleanup() {
	h.overlayManager.Clear()

	if h.disableEventTap != nil {
		h.disableEventTap()
	}

	h.appState.SetMode(domain.ModeIdle)
	h.logger.Debug("Mode transition complete",
		zap.String("to", "idle"))
	h.overlayManager.SwitchTo(overlay.ModeIdle)

	// If a hotkey refresh was deferred while in an active mode, perform it now
	if h.appState.HotkeyRefreshPending() {
		h.appState.SetHotkeyRefreshPending(false)

		if h.refreshHotkeys != nil {
			go h.refreshHotkeys()
		}
	}
}

// handleCursorRestoration handles cursor position restoration on exit.
func (h *Handler) handleCursorRestoration() {
	shouldRestore := h.shouldRestoreCursorOnExit()
	if shouldRestore {
		currentBounds := bridge.ActiveScreenBounds()
		target := coordinates.ComputeRestoredPosition(
			h.cursorState.InitialPosition(),
			h.cursorState.InitialScreenBounds(),
			currentBounds,
		)
		ctx := context.Background()

		restoreCursorErr := h.actionService.MoveCursorToPoint(ctx, target)
		if restoreCursorErr != nil {
			h.logger.Error("Failed to restore cursor position", zap.Error(restoreCursorErr))
		}
	}

	h.cursorState.Reset()
	// Always reset scroll context regardless of whether we performed cursor restoration.
	// This ensures proper state cleanup when switching between modes.
	h.scroll.Context.SetIsActive(false)
	h.scroll.Context.SetLastKey("")
}

// CurrModeString returns the current mode as a string.
func (h *Handler) CurrModeString() string {
	return domain.ModeString(h.appState.CurrentMode())
}

// CaptureInitialCursorPosition captures the initial cursor position and screen bounds.
func (h *Handler) CaptureInitialCursorPosition() {
	if h.cursorState.IsCaptured() {
		return
	}

	ctx := context.Background()

	pos, posErr := h.actionService.CursorPosition(ctx)
	if posErr != nil {
		h.logger.Error("Failed to get cursor position", zap.Error(posErr))

		return
	}

	screenBounds := bridge.ActiveScreenBounds()
	h.cursorState.Capture(pos, screenBounds)
}

// shouldRestoreCursorOnExit determines if the cursor should be restored on mode exit.
func (h *Handler) shouldRestoreCursorOnExit() bool {
	if h.config == nil {
		return false
	}

	if !h.config.General.RestoreCursorPosition {
		return false
	}

	if !h.cursorState.IsCaptured() {
		return false
	}

	if h.scroll.Context.IsActive() {
		return false
	}

	return h.cursorState.ShouldRestore()
}

// handleActionKey handles action keys for both hints and grid modes.

func (h *Handler) handleActionKey(key string, mode string) {
	ctx := context.Background()
	h.actionService.HandleActionKey(ctx, key, mode)
}

// overlaySwitch switches the overlay mode.
func (h *Handler) overlaySwitch(m overlay.Mode) {
	if h.overlayManager != nil {
		h.overlayManager.SwitchTo(m)
	}
}

// SetModeIdle switches the application to idle mode, disabling active navigation modes.
// This function resets the application state to idle, disables event tapping,
// and switches the overlay display to the idle state.
func (h *Handler) SetModeIdle() {
	h.appState.SetMode(domain.ModeIdle)

	if h.disableEventTap != nil {
		h.disableEventTap()
	}

	h.overlaySwitch(overlay.ModeIdle)
}

// setMode sets the application mode, enables event tap, and switches overlay.
func (h *Handler) setMode(appMode domain.Mode, overlayMode overlay.Mode) {
	h.appState.SetMode(appMode)

	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	h.overlaySwitch(overlayMode)
}

// activateModeBase performs common activation steps for all modes.
func (h *Handler) activateModeBase(
	modeName string,
	enabled bool,
	actionEnum domain.Action,
) (domain.Action, bool) {
	// Validate mode activation
	err := h.validateModeActivation(modeName, enabled)
	if err != nil {
		h.logger.Warn(modeName+" mode activation failed", zap.Error(err))

		return domain.ActionMoveMouse, false
	}

	// Prepare for mode activation (reset scroll, capture cursor)
	h.prepareForModeActivation()

	actionString := domain.ActionString(actionEnum)
	h.logger.Info("Activating "+modeName+" mode", zap.String("action", actionString))

	// Always resize overlay to the active screen
	h.overlayManager.ResizeToActiveScreenSync()

	return actionEnum, true
}

// SetModeHints switches the application to hints mode for accessibility-based navigation.
// This function sets the application state to hints mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to hints mode.
func (h *Handler) SetModeHints() {
	h.setMode(domain.ModeHints, overlay.ModeHints)
}

// SetModeGrid switches the application to grid mode for coordinate-based navigation.
// This function sets the application state to grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to grid mode.
func (h *Handler) SetModeGrid() {
	h.setMode(domain.ModeGrid, overlay.ModeGrid)
}
