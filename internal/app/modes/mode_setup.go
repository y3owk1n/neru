package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// CurrModeString returns the current mode as a string.
func (h *Handler) CurrModeString() string {
	return domain.ModeString(h.appState.CurrentMode())
}

// CaptureInitialCursorPosition captures the initial cursor position and screen bounds.
func (h *Handler) CaptureInitialCursorPosition() {
	if h.config == nil {
		return
	}

	if !h.config.General.RestoreCursorPosition && !h.config.General.CenterCursorPosition {
		return
	}

	if h.cursorState.IsCaptured() {
		return
	}

	ctx := context.Background()

	pos, posErr := h.actionService.CursorPosition(ctx)
	if posErr != nil {
		h.logger.Error("Failed to get cursor position", zap.Error(posErr))

		return
	}

	var screenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(context.Background())
		if err == nil {
			screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for cursor capture", zap.Error(err))
		}
	}

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

	if h.scroll != nil && h.scroll.Context != nil && h.scroll.Context.IsActive() {
		return false
	}

	return h.cursorState.ShouldMoveCursor()
}

// shouldCenterCursorOnExit determines if the cursor should be centered on mode exit.
func (h *Handler) shouldCenterCursorOnExit() bool {
	if h.config == nil {
		return false
	}

	if !h.config.General.CenterCursorPosition {
		return false
	}

	if h.scroll != nil && h.scroll.Context != nil && h.scroll.Context.IsActive() {
		return false
	}

	return h.cursorState.ShouldMoveCursor()
}

// overlaySwitch switches the overlay mode.
func (h *Handler) overlaySwitch(m overlay.Mode) {
	if h.overlayManager != nil {
		h.overlayManager.SwitchTo(m)
	}
}

func (h *Handler) setAppModeLocked(mode domain.Mode) {
	h.modeSession++
	h.appState.SetMode(mode)

	// Reset modifier state BEFORE enabling detection.
	// This ensures any modifiers from the activation hotkey (e.g., Cmd+Shift+Space)
	// don't get picked up as sticky modifiers.
	if h.modifierState != nil {
		h.modifierState.Reset()
	}

	// Cancel any pending modifier toggle from the activation hotkey.
	h.cancelPendingModifierToggle()

	// Disarm modifier detection until all modifiers have been released.
	// This prevents hotkey modifiers (e.g., Cmd+Shift from the activation combo)
	// or IPC tool key releases from being misinterpreted as intentional taps.
	// Detection is re-armed when we see a _up event with no modifiers held.
	h.modifierDetectionArmed = false

	h.syncModifierPassthrough(mode)
	h.syncStickyModifierToggle(mode)
}

func (h *Handler) syncStickyModifierToggle(mode domain.Mode) {
	if h.setStickyModifierToggle == nil {
		return
	}

	isNavMode := mode == domain.ModeHints ||
		mode == domain.ModeGrid ||
		mode == domain.ModeRecursiveGrid ||
		mode == domain.ModeScroll

	enabled := isNavMode && h.config != nil && h.config.StickyModifiers.Enabled

	h.setStickyModifierToggle(enabled)
}

// SetModeIdle switches the application to idle mode, disabling active navigation modes.
// This function resets the application state to idle, disables event tapping,
// and switches the overlay display to the idle state.
//
// NOTE: Every code path that calls appState.SetMode() must also call
// syncModifierPassthrough() with the same mode to keep the event tap
// passthrough state consistent. See also: performCommonCleanup, setMode.
func (h *Handler) SetModeIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setAppModeLocked(domain.ModeIdle)

	if h.disableEventTap != nil {
		h.disableEventTap()
	}

	h.overlaySwitch(overlay.ModeIdle)
}

// setModeLocked sets the application mode, enables event tap, and switches overlay.
// Caller must hold h.mu.
func (h *Handler) setModeLocked(appMode domain.Mode, overlayMode overlay.Mode) {
	h.setAppModeLocked(appMode)

	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	h.overlaySwitch(overlayMode)
}

// activateModeBase performs common activation steps for all modes.
func (h *Handler) activateModeBase(
	modeName string,
	enabled bool,
	actionEnum action.Type,
) (action.Type, bool) {
	// Validate mode activation
	err := h.validateModeActivation(modeName, enabled)
	if err != nil {
		h.logger.Warn(modeName+" mode activation failed", zap.Error(err))

		return action.TypeMoveMouse, false
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
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setModeLocked(domain.ModeHints, overlay.ModeHints)
}

// SetModeGrid switches the application to grid mode for coordinate-based navigation.
// This function sets the application state to grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to grid mode.
func (h *Handler) SetModeGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setModeLocked(domain.ModeGrid, overlay.ModeGrid)
}

// SetModeRecursiveGrid switches the application to recursive-grid mode for recursive cell navigation.
// This function sets the application state to recursive-grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to recursive-grid mode.
func (h *Handler) SetModeRecursiveGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setModeLocked(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)
}

// SetModeScroll switches the application to scroll mode for scroll-based navigation.
// This function sets the application state to scroll mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to scroll mode.
func (h *Handler) SetModeScroll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setModeLocked(domain.ModeScroll, overlay.ModeScroll)
}
