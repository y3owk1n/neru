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
// Per-mode keys are merged on top of global keys (additive). If no per-mode keys
// are configured, the global keys are returned as-is.
func (h *Handler) resolveExitKeysForCurrentMode() []string {
	globalKeys := h.config.General.ModeExitKeys
	if len(globalKeys) == 0 {
		globalKeys = DefaultModeExitKeys()
	}

	var modeKeys []string

	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		modeKeys = h.config.Hints.ModeExitKeys
	case domain.ModeGrid:
		modeKeys = h.config.Grid.ModeExitKeys
	case domain.ModeRecursiveGrid:
		modeKeys = h.config.RecursiveGrid.ModeExitKeys
	case domain.ModeScroll:
		modeKeys = h.config.Scroll.ModeExitKeys
	case domain.ModeIdle:
		// Idle mode has no per-mode exit keys
	}

	return config.MergeExitKeys(globalKeys, modeKeys)
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
