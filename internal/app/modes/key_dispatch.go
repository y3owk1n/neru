package modes

import (
	"github.com/y3owk1n/neru/internal/config"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Determine escape/exit keys from config with sensible defaults
	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = DefaultModeExitKeys()
	}

	// Check if key matches any configured exit keys (after normalization)
	if config.IsExitKey(key, exitKeys) {
		h.handleEscapeKey()

		return
	}

	h.handleModeSpecificKey(key)
}

// handleEscapeKey handles the escape key to exit the current mode.
// Caller must hold h.mu.
func (h *Handler) handleEscapeKey() {
	_, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	h.exitModeLocked()
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
