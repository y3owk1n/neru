package modes

import (
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// DEBUG: log every key event to trace modifier down/up flow
	h.logger.Info("HandleKeyPress",
		zap.String("key", key),
		zap.String("mode", h.CurrModeString()),
		zap.String("pendingMod", h.pendingModifierKey),
		zap.String("stickyMods", h.stickyModifiers().String()))

	// Cancel any pending modifier toggle if a non-modifier key is pressed
	// This handles the case where Shift+L is pressed - the modifier tap
	// is canceled when L comes in
	if h.pendingModifierKey != "" && !strings.HasPrefix(key, modifierTogglePrefix) {
		h.cancelPendingModifierToggle()
	}

	// Check for modifier toggle keys before any other processing
	if h.handleModifierToggle(key) {
		return
	}

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
