package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	if h.AppState.CurrentMode() == domain.ModeIdle {
		if key == "\x1b" || key == "escape" {
			h.Logger.Info("Exiting standalone scroll mode")
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()

			if h.DisableEventTap != nil {
				h.DisableEventTap()
			}

			h.Scroll.Context.SetIsActive(false)
			h.Scroll.Context.SetLastKey("")
			// Reset cursor state when exiting scroll mode to ensure proper cursor restoration
			// in subsequent modes
			h.CursorState.Reset()

			return
		}
		// Try to handle scroll keys with generic handler using persistent state.
		// If it's not a scroll key, it will just be ignored.
		lastKey := h.Scroll.Context.LastKey
		h.handleGenericScrollKey(key, &lastKey)
		h.Scroll.Context.SetLastKey(lastKey)

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
	switch h.AppState.CurrentMode() {
	case domain.ModeHints:
		// Skip tab handling if pending action is set
		if h.Hints.Context.GetPendingAction() != nil {
			h.Logger.Debug("Tab key disabled when action is pending")

			return
		}

		if h.Hints.Context.InActionMode {
			h.Hints.Context.SetInActionMode(false)

			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			// Re-activate hint mode while preserving action mode state
			h.activateHintModeInternal(true, nil)
			h.Logger.Info("Switched back to hints overlay mode")
			h.overlaySwitch(overlay.ModeHints)
		} else {
			h.Hints.Context.SetInActionMode(true)
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()
			h.drawActionHighlight()
			h.OverlayManager.Show()
			h.Logger.Info("Switched to hints action mode")
			h.overlaySwitch(overlay.ModeAction)
		}
	case domain.ModeGrid:
		// Skip tab handling if pending action is set
		if h.Grid.Context.GetPendingAction() != nil {
			h.Logger.Debug("Tab key disabled when action is pending")

			return
		}

		if h.Grid.Context.InActionMode {
			h.Grid.Context.SetInActionMode(false)
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()

			// Re-activate grid mode (similar to hints mode pattern)
			h.activateGridModeWithAction(nil)

			h.Logger.Info("Switched back to grid overlay mode")
			h.overlaySwitch(overlay.ModeGrid)
		} else {
			h.Grid.Context.SetInActionMode(true)
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()
			h.drawActionHighlight()
			h.OverlayManager.Show()
			h.Logger.Info("Switched to grid action mode")
			h.overlaySwitch(overlay.ModeAction)
		}
	case domain.ModeIdle:
		return
	}
}

// handleEscapeKey handles the escape key to exit action mode or current mode.
func (h *Handler) handleEscapeKey() {
	switch h.AppState.CurrentMode() {
	case domain.ModeHints:
		if h.Hints.Context.InActionMode {
			h.Hints.Context.SetInActionMode(false)
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()
			h.ExitMode()
			h.Logger.Info("Exited hints action mode completely")
			h.overlaySwitch(overlay.ModeIdle)

			return
		}
	case domain.ModeGrid:
		if h.Grid.Context.InActionMode {
			h.Grid.Context.SetInActionMode(false)
			h.OverlayManager.Clear()
			h.OverlayManager.Hide()
			h.ExitMode()
			h.Logger.Info("Exited grid action mode completely")
			h.overlaySwitch(overlay.ModeIdle)

			return
		}
	case domain.ModeIdle:
		return
	}

	h.ExitMode()
	h.SetModeIdle()
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *Handler) handleModeSpecificKey(key string) {
	switch h.AppState.CurrentMode() {
	case domain.ModeHints:
		if h.Hints.Context.InActionMode {
			h.handleHintsActionKey(key)
			// After handling the action, we stay in action mode.
			// The user can press Tab to go back to overlay mode or perform more actions.
			return
		}

		// Route hint-specific keys via domain hints router
		if h.Hints.Context.Router == nil {
			h.Logger.Error("Hints router is nil")
			h.ExitMode()

			return
		}

		hintKeyResult := h.Hints.Context.Router.RouteKey(key)
		if hintKeyResult.Exit {
			h.ExitMode()

			return
		}

		// Hint input processed by router; if exact match, perform action
		if hintKeyResult.ExactHint != nil {
			hint := hintKeyResult.ExactHint
			// Use the domain element's center point
			center := hint.Element().Center()

			h.Logger.Info("Found element", zap.String("label", hint.Label()))

			ctx := context.Background()

			moveCursorErr := h.ActionService.MoveCursorToPoint(ctx, center)
			if moveCursorErr != nil {
				h.Logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Check if there's a pending action to execute
			pendingAction := h.Hints.Context.GetPendingAction()
			if pendingAction != nil {
				h.Logger.Info("Executing pending action", zap.String("action", *pendingAction))
				// Use ActionService
				ctx := context.Background()

				performActionErr := h.ActionService.PerformAction(ctx, *pendingAction, center)
				if performActionErr != nil {
					h.Logger.Error("Failed to perform pending action", zap.Error(performActionErr))
				}
				// Exit mode after executing action
				h.ExitMode()

				return
			}

			// No pending action - re-activate hints mode to show hints again
			h.Logger.Info("Re-activating hints mode after cursor movement")
			h.activateHintModeInternal(false, nil)

			return
		}
	case domain.ModeGrid:
		if h.Grid.Context.InActionMode {
			h.handleGridActionKey(key)
			// After handling the action, we stay in action mode.
			// The user can press Tab to go back to overlay mode or perform more actions.
			return
		}

		gridKeyResult := h.Grid.Router.RouteKey(key)
		if gridKeyResult.Exit {
			h.ExitMode()

			return
		}

		if gridKeyResult.Complete {
			targetPoint := gridKeyResult.TargetPoint

			// Convert from window-local coordinates to absolute screen coordinates using helper
			screenBounds := bridge.GetActiveScreenBounds()
			absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, screenBounds)

			h.Logger.Info(
				"Grid move mouse",
				zap.Int("x", absolutePoint.X),
				zap.Int("y", absolutePoint.Y),
			)

			ctx := context.Background()

			moveCursorErr := h.ActionService.MoveCursorToPoint(ctx, absolutePoint)
			if moveCursorErr != nil {
				h.Logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Check if there's a pending action to execute
			pendingAction := h.Grid.Context.GetPendingAction()
			if pendingAction != nil {
				h.Logger.Info("Executing pending action", zap.String("action", *pendingAction))
				// Use ActionService
				ctx := context.Background()

				performActionErr := h.ActionService.PerformAction(
					ctx,
					*pendingAction,
					absolutePoint,
				)
				if performActionErr != nil {
					h.Logger.Error("Failed to perform pending action", zap.Error(performActionErr))
				}
				// Exit mode after executing action
				h.ExitMode()

				return
			}

			// No need to exit grid mode, just let it going

			return
		}
	case domain.ModeIdle:
		return
	}
}

// ExitMode exits the current mode.
func (h *Handler) ExitMode() {
	if h.AppState.CurrentMode() == domain.ModeIdle {
		return
	}

	h.Logger.Info("Exiting current mode", zap.String("mode", h.GetCurrModeString()))

	h.performModeSpecificCleanup()
	h.performCommonCleanup()
	h.handleCursorRestoration()
}

// performModeSpecificCleanup handles mode-specific cleanup logic.
func (h *Handler) performModeSpecificCleanup() {
	switch h.AppState.CurrentMode() {
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

// cleanupHintsMode handles cleanup for hints mode.
func (h *Handler) cleanupHintsMode() {
	h.Hints.Context.SetInActionMode(false)
	h.Hints.Context.Reset()

	h.OverlayManager.Clear()
	h.OverlayManager.Hide()
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
	h.Grid.Context.SetInActionMode(false)

	if h.Grid.Manager != nil {
		h.Grid.Manager.Reset()
	}

	h.OverlayManager.Clear()
	h.OverlayManager.Hide()
}

// performCommonCleanup handles common cleanup logic for all modes.
func (h *Handler) performCommonCleanup() {
	h.OverlayManager.Clear()

	if h.DisableEventTap != nil {
		h.DisableEventTap()
	}

	h.AppState.SetMode(domain.ModeIdle)
	h.Logger.Debug("Mode transition complete",
		zap.String("to", "idle"))
	h.OverlayManager.SwitchTo(overlay.ModeIdle)

	// If a hotkey refresh was deferred while in an active mode, perform it now
	if h.AppState.HotkeyRefreshPending() {
		h.AppState.SetHotkeyRefreshPending(false)

		if h.RefreshHotkeys != nil {
			go h.RefreshHotkeys()
		}
	}
}

// handleCursorRestoration handles cursor position restoration on exit.
func (h *Handler) handleCursorRestoration() {
	shouldRestore := h.shouldRestoreCursorOnExit()
	if shouldRestore {
		currentBounds := bridge.GetActiveScreenBounds()
		target := coordinates.ComputeRestoredPosition(
			h.CursorState.GetInitialPosition(),
			h.CursorState.GetInitialScreenBounds(),
			currentBounds,
		)
		ctx := context.Background()

		restoreCursorErr := h.ActionService.MoveCursorToPoint(ctx, target)
		if restoreCursorErr != nil {
			h.Logger.Error("Failed to restore cursor position", zap.Error(restoreCursorErr))
		}
	}

	h.CursorState.Reset()
	// Always reset scroll context regardless of whether we performed cursor restoration.
	// This ensures proper state cleanup when switching between modes.
	h.Scroll.Context.SetIsActive(false)
	h.Scroll.Context.SetLastKey("")
}

// GetCurrModeString returns the current mode as a string.
func (h *Handler) GetCurrModeString() string {
	return domain.GetModeString(h.AppState.CurrentMode())
}

// CaptureInitialCursorPosition captures the initial cursor position and screen bounds.
func (h *Handler) CaptureInitialCursorPosition() {
	if h.CursorState.IsCaptured() {
		return
	}

	context := context.Background()

	pos, posErr := h.ActionService.GetCursorPosition(context)
	if posErr != nil {
		h.Logger.Error("Failed to get cursor position", zap.Error(posErr))

		return
	}

	screenBounds := bridge.GetActiveScreenBounds()
	h.CursorState.Capture(pos, screenBounds)
}

// shouldRestoreCursorOnExit determines if the cursor should be restored on mode exit.
func (h *Handler) shouldRestoreCursorOnExit() bool {
	if h.Config == nil {
		return false
	}

	if !h.Config.General.RestoreCursorPosition {
		return false
	}

	if !h.CursorState.IsCaptured() {
		return false
	}

	if h.Scroll.Context.GetIsActive() {
		return false
	}

	return h.CursorState.ShouldRestore()
}

// handleActionKey handles action keys for both hints and grid modes.
func (h *Handler) handleActionKey(key string, mode string) {
	context := context.Background()

	cursorPos, cursorPosErr := h.ActionService.GetCursorPosition(context)
	if cursorPosErr != nil {
		h.Logger.Error("Failed to get cursor position", zap.Error(cursorPosErr))

		return
	}

	var act string

	switch key {
	case h.Config.Action.LeftClickKey:
		h.Logger.Info(mode + " action: Left click")

		act = string(domain.ActionNameLeftClick)
	case h.Config.Action.RightClickKey:
		h.Logger.Info(mode + " action: Right click")

		act = string(domain.ActionNameRightClick)
	case h.Config.Action.MiddleClickKey:
		h.Logger.Info(mode + " action: Middle click")

		act = string(domain.ActionNameMiddleClick)
	case h.Config.Action.MouseDownKey:
		h.Logger.Info(mode + " action: Mouse down")

		act = string(domain.ActionNameMouseDown)
	case h.Config.Action.MouseUpKey:
		h.Logger.Info(mode + " action: Mouse up")

		act = string(domain.ActionNameMouseUp)
	default:
		h.Logger.Debug("Unknown "+mode+" action key", zap.String("key", key))

		return
	}

	// Use ActionService
	performActionErr := h.ActionService.PerformAction(context, act, cursorPos)
	if performActionErr != nil {
		h.Logger.Error("Failed to perform action", zap.Error(performActionErr))
	}
}

// overlaySwitch switches the overlay mode.
func (h *Handler) overlaySwitch(m overlay.Mode) {
	if h.OverlayManager != nil {
		h.OverlayManager.SwitchTo(m)
	}
}

// SetModeIdle switches the application to idle mode, disabling active navigation modes.
// This function resets the application state to idle, disables event tapping,
// and switches the overlay display to the idle state.
func (h *Handler) SetModeIdle() {
	h.AppState.SetMode(domain.ModeIdle)

	if h.DisableEventTap != nil {
		h.DisableEventTap()
	}

	h.overlaySwitch(overlay.ModeIdle)
}

// SetModeHints switches the application to hints mode for accessibility-based navigation.
// This function sets the application state to hints mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to hints mode.
func (h *Handler) SetModeHints() {
	h.AppState.SetMode(domain.ModeHints)

	if h.EnableEventTap != nil {
		h.EnableEventTap()
	}

	h.overlaySwitch(overlay.ModeHints)
}

// SetModeGrid switches the application to grid mode for coordinate-based navigation.
// This function sets the application state to grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to grid mode.
func (h *Handler) SetModeGrid() {
	h.AppState.SetMode(domain.ModeGrid)

	if h.EnableEventTap != nil {
		h.EnableEventTap()
	}

	h.overlaySwitch(overlay.ModeGrid)
}
