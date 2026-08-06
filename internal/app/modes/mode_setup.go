package modes

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// CurrModeString returns the current mode as a string.
func (h *handlerState) CurrModeString() string {
	return domain.ModeString(h.appState.CurrentMode())
}

// overlaySwitch switches the overlay mode.
func (h *handlerState) overlaySwitch(m overlay.Mode) {
	if h.overlayManager != nil {
		h.overlayManager.SwitchTo(m)
	}
}

func (h *handlerState) setAppMode(mode domain.Mode) {
	h.modeSession++
	h.appState.SetMode(mode)

	// Reset sticky modifier state before enabling detection for the new session.
	// Activation-hotkey modifiers are suppressed explicitly by the hotkey path.
	if h.modifierState != nil {
		h.clearStickyModifiers()
	}

	// Cancel any pending modifier tap state from the previous mode session.
	h.cancelPendingModifierToggle()

	h.syncModifierPassthrough(mode)
	h.syncStickyModifierToggle(mode)
}

func (h *handlerState) syncStickyModifierToggle(mode domain.Mode) {
	if !h.hasEventTap() {
		return
	}

	isNavMode := mode == domain.ModeHints ||
		mode == domain.ModeGrid ||
		mode == domain.ModeRecursiveGrid ||
		mode == domain.ModeScroll ||
		mode == domain.ModeMonitorSelect

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

	h.setAppMode(domain.ModeIdle)

	if h.hasEventTap() {
		h.disableEventTap()
	}

	h.overlaySwitch(overlay.ModeIdle)
}

// enterMode sets the application mode and enables the event tap, without
// touching the overlay. A mode that hands over a Frame gets its overlay shown
// and switched when the Frame is realized, and doing it here as well would put
// a second owner on a sequence that has exactly one.
// Caller must hold h.mu.
func (h *handlerState) enterMode(appMode domain.Mode) {
	h.setAppMode(appMode)

	if h.hasEventTap() {
		h.enableEventTap()
	}
}

// setMode enters a mode and switches the overlay to it. It is what the modes
// that still draw through the manager use; the converted ones call enterMode
// and hand over a Frame.
// Caller must hold h.mu.
func (h *handlerState) setMode(appMode domain.Mode, overlayMode overlay.Mode) {
	h.enterMode(appMode)

	h.overlaySwitch(overlayMode)
}

// activateModeBase performs common activation steps for all modes.
func (h *handlerState) activateModeBase(
	modeName string,
	enabled bool,
	actionEnum action.Type,
	bundleID string,
) (action.Type, bool) {
	err := h.validateModeActivation(bundleID, modeName, enabled)
	if err != nil {
		h.logger.Warn(modeName+" mode activation failed", zap.Error(err))

		return action.TypeMoveMouse, false
	}

	// Prepare for mode activation (reset transient mode state)
	h.prepareForModeActivation()

	actionString := domain.ActionString(actionEnum)
	h.logger.Debug("Activating "+modeName+" mode", zap.String("action", actionString))

	// Always resize overlay to the active screen
	if h.overlayManager != nil {
		h.overlayManager.ResizeToActiveScreen()
	}

	return actionEnum, true
}

// SetModeHints switches the application to hints mode for accessibility-based
// navigation. This function sets the application state to hints mode and
// enables event tapping for capturing keyboard input. The overlay comes up
// when the mode hands over its Frame, not here.
func (h *Handler) SetModeHints() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.enterMode(domain.ModeHints)
}

// SetModeGrid switches the application to grid mode for coordinate-based navigation.
// This function sets the application state to grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to grid mode.
func (h *Handler) SetModeGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setMode(domain.ModeGrid, overlay.ModeGrid)
}

// SetModeRecursiveGrid switches the application to recursive-grid mode for recursive cell navigation.
// This function sets the application state to recursive-grid mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to recursive-grid mode.
func (h *Handler) SetModeRecursiveGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setMode(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)
}

// SetModeScroll switches the application to scroll mode for scroll-based navigation.
// This function sets the application state to scroll mode, enables event tapping
// for capturing keyboard input, and switches the overlay display to scroll mode.
func (h *Handler) SetModeScroll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setMode(domain.ModeScroll, overlay.ModeScroll)
}
