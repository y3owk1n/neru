package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	if key == KeyTab {
		h.handleTabKey()

		return
	}

	if key == KeyEscape || key == KeyEscape2 {
		h.handleEscapeKey()

		return
	}

	// Check if we're in action mode and delegate to the mode
	mode, exists := h.modes[h.appState.CurrentMode()]
	if exists {
		switch h.appState.CurrentMode() {
		case domain.ModeHints:
			if h.hints.Context.InActionMode() {
				mode.HandleActionKey(key)

				return
			}
		case domain.ModeGrid:
			if h.grid.Context.InActionMode() {
				mode.HandleActionKey(key)

				return
			}
		case domain.ModeScroll:
			// Scroll mode doesn't have action modes
		case domain.ModeAction:
			// Action mode is always in action mode
			mode.HandleActionKey(key)

			return
		case domain.ModeIdle:
			// No action mode for idle
		default:
			// No action mode for other modes
		}
	}

	h.handleModeSpecificKey(key)
}

// handleTabKey handles the tab key to toggle between overlay mode and action mode.
func (h *Handler) handleTabKey() {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.ToggleActionMode()
}

// exitActionModeForContext exits action mode for the given context and mode name.
func (h *Handler) exitActionModeForContext(ctx actionModeContext, modeName string) {
	ctx.SetInActionMode(false)
	h.overlayManager.Clear()
	h.overlayManager.Hide()
	h.ExitMode()
	h.logger.Info("Exited " + modeName + " action mode completely")
	h.overlaySwitch(overlay.ModeIdle)
}

// handleEscapeKey handles the escape key to exit action mode or current mode.
func (h *Handler) handleEscapeKey() {
	_, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	// Check if we're in action mode for this specific mode
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints.Context.InActionMode() {
			h.exitActionModeForContext(h.hints.Context, "hints")

			return
		}
	case domain.ModeGrid:
		if h.grid.Context.InActionMode() {
			h.exitActionModeForContext(h.grid.Context, "grid")

			return
		}
	case domain.ModeScroll:
		// Scroll mode doesn't have action modes, just exit
	case domain.ModeAction:
		// Action mode doesn't have sub-modes, just exit
	case domain.ModeIdle:
		// No action mode for idle
	default:
		// No action mode for other modes
	}

	h.ExitMode()
	h.SetModeIdle()
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *Handler) handleModeSpecificKey(key string) {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.HandleKey(key)
}
