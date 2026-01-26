package modes

import (
	"github.com/y3owk1n/neru/internal/config"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	// Process any pending hints refresh from timer callback (dispatched to main thread)
	select {
	case <-h.refreshHintsCh:
		h.activateHintModeInternal(false, nil)
	default:
		// No pending refresh
	}

	// Determine escape/exit keys from config with sensible defaults
	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = []string{KeyEscape, KeyEscape2}
	}

	// Normalize incoming key for comparison
	normalizedKey := config.NormalizeKeyForComparison(key)

	// Check if key matches any configured exit keys (after normalization)
	for _, exitKey := range exitKeys {
		normalizedExitKey := config.NormalizeKeyForComparison(exitKey)
		if normalizedKey == normalizedExitKey {
			h.handleEscapeKey()

			return
		}
	}

	h.handleModeSpecificKey(key)
}

// handleEscapeKey handles the escape key to exit the current mode.
func (h *Handler) handleEscapeKey() {
	_, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
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
