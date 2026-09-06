package modes

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// CurrModeString returns the current mode as a string.
func (h *handlerState) CurrModeString() string {
	return domain.ModeString(h.appState.CurrentMode())
}

func (h *handlerState) setAppMode(mode domain.Mode) {
	h.modeSession++
	h.appState.SetMode(mode)

	// The declared name belongs to a ModeCustom session and to nothing else.
	// A declared mode sets it before entering, so it survives this call on
	// the way in and is cleared on the way to any other mode.
	if mode != domain.ModeCustom {
		h.customModeName = ""
	}

	// Reset sticky modifier state before enabling detection for the new session.
	// Activation-hotkey modifiers are suppressed explicitly by the hotkey path.
	if h.modifierState != nil {
		h.clearStickyModifiers()
	}

	// Cancel any pending modifier tap state from the previous mode session.
	h.cancelPendingModifierToggle()

	// Settle the keymap for the mode being entered before anything reads it.
	// Entering a mode is a one-shot the user is waiting on, which is where
	// learning the focused app from the platform is allowed to happen; doing it
	// here is what leaves the keystrokes after it with nothing to ask (ADR 0005).
	h.settledKeymap()

	h.syncModifierPassthrough(mode)
	h.syncStickyModifierToggle(mode)
}

func (h *handlerState) syncStickyModifierToggle(mode domain.Mode) {
	if !h.hasEventTap() {
		return
	}

	// Every mode but idle captures the keyboard, and sticky modifiers are a
	// captured keyboard's to toggle.
	isNavMode := mode != domain.ModeIdle

	enabled := isNavMode && h.config != nil && h.config.StickyModifiers.Enabled

	h.setStickyModifierToggle(enabled)
}

// SetModeIdle switches the application to idle mode, disabling active navigation modes.
// This function resets the application state to idle, disables event tapping,
// and takes whatever was on screen off it.
//
// NOTE: Every code path that calls appState.SetMode() must also call
// syncModifierPassthrough() with the same mode to keep the event tap
// passthrough state consistent. See also: performCommonCleanup, enterMode.
func (h *Handler) SetModeIdle() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.setAppMode(domain.ModeIdle)

	if h.hasEventTap() {
		h.disableEventTap()
	}

	h.clearOverlayFrame()
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

	// The overlay is not sized here. Every mode that reaches this hands over a
	// Frame, and sizing to the active screen is the first step of realizing
	// one; doing it here as well is a second trip to the main thread for a
	// screen the frame is about to ask for anyway.

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

// SetModeGrid switches the application to grid mode for coordinate-based
// navigation. This function sets the application state to grid mode and
// enables event tapping for capturing keyboard input. The overlay comes up
// when the mode hands over its Frame, not here.
func (h *Handler) SetModeGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.enterMode(domain.ModeGrid)
}

// SetModeRecursiveGrid switches the application to recursive-grid mode for
// recursive cell navigation. This function sets the application state to
// recursive-grid mode and enables event tapping for capturing keyboard input.
// The overlay comes up when the mode hands over its Frame, not here.
func (h *Handler) SetModeRecursiveGrid() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.enterMode(domain.ModeRecursiveGrid)
}

// SetModeScroll switches the application to scroll mode for scroll-based
// navigation. This function sets the application state to scroll mode and
// enables event tapping for capturing keyboard input. The overlay comes up
// when the mode hands over its Frame, not here.
func (h *Handler) SetModeScroll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.enterMode(domain.ModeScroll)
}
