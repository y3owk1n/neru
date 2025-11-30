package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

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

	if h.scroll != nil && h.scroll.Context != nil && h.scroll.Context.IsActive() {
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
	if h.overlayManager != nil {
		h.overlayManager.ResizeToActiveScreen()
	}

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

// SetModeScroll switches the application to scroll mode for scroll-based navigation.
// This function sets the application state to scroll mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to scroll mode.
func (h *Handler) SetModeScroll() {
	h.setMode(domain.ModeScroll, overlay.ModeAction)
}

// SetModeAction switches the application to action mode for action-based operations.
// This function sets the application state to action mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to action mode.
func (h *Handler) SetModeAction() {
	h.setMode(domain.ModeAction, overlay.ModeAction)
}
