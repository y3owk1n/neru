package modes

import (
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Cancel any pending modifier toggle if a non-modifier key is pressed
	// This handles the case where Shift+L is pressed - the modifier tap
	// is canceled when L comes in
	if !strings.HasPrefix(key, modifierTogglePrefix) {
		h.lastRegularKeyTime = time.Now()

		h.cancelPendingModifierToggle()
	}

	// Check for modifier toggle keys before any other processing
	if h.handleModifierToggle(key) {
		return
	}

	// Since sticky modifiers are injected as physical events, they appear in "key"
	// (e.g. Cmd+Shift+L). This strips them out so bindings like "Shift+L" still match.
	activeMods := h.stickyModifiers()
	if activeMods != 0 && !strings.HasPrefix(key, modifierTogglePrefix) {
		key = h.stripStickyModifiersFromKey(key, activeMods)
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

// stripStickyModifiersFromKey removes any currently active sticky modifiers from the
// incoming key string so that physical injections don't break expected key bindings.
func (h *Handler) stripStickyModifiersFromKey(key string, mods action.Modifiers) string {
	parts := strings.Split(key, "+")
	if len(parts) <= 1 {
		return key
	}

	var newParts []string

	for i, part := range parts {
		// Only check for modifiers on the prefixes
		if i < len(parts)-1 {
			lowerPart := strings.ToLower(part)

			if lowerPart == "cmd" && mods.Has(action.ModCmd) {
				continue
			}

			if lowerPart == "shift" && mods.Has(action.ModShift) {
				continue
			}

			if lowerPart == "alt" && mods.Has(action.ModAlt) {
				continue
			}

			if lowerPart == "ctrl" && mods.Has(action.ModCtrl) {
				continue
			}

			if lowerPart == "option" && mods.Has(action.ModAlt) {
				continue
			}
		}

		newParts = append(newParts, part)
	}

	return strings.Join(newParts, "+")
}
