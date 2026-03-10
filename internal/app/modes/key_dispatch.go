package modes

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Resolve exit keys for the current mode (global + per-mode, merged)
	exitKeys := h.resolveExitKeysForCurrentMode()

	// Check if key matches any configured exit keys (after normalization)
	if config.IsExitKey(key, exitKeys) {
		h.handleEscapeKey()

		return
	}

	h.handleModeSpecificKey(key)
}

// resolveExitKeysForCurrentMode returns the effective exit keys for the current mode.
// It delegates to Config.ResolvedExitKeys to keep a single resolution path for all callers.
func (h *Handler) resolveExitKeysForCurrentMode() []string {
	return h.config.ResolvedExitKeys(domain.ModeString(h.appState.CurrentMode()))
}

// handleEscapeKey handles the escape key to exit the current mode.
// Caller must hold h.mu.
func (h *Handler) handleEscapeKey() {
	_, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	h.exitModeLocked()
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *Handler) handleModeSpecificKey(key string) {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.HandleKey(key)
}
