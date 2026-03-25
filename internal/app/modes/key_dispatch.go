package modes

import (
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const customHotkeySequenceTimeout = 500 * time.Millisecond

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

	// Save the raw key before sticky modifier stripping so we can try
	// custom hotkey matching with the original modifier combo later.
	rawKey := key

	// Since sticky modifiers are injected as physical events, they appear in "key"
	// (e.g. Cmd+Shift+L). This strips them out so bindings like "Shift+L" still match.
	activeMods := h.stickyModifiers()
	if activeMods != 0 && !strings.HasPrefix(key, modifierTogglePrefix) {
		key = h.stripStickyModifiersFromKey(key, activeMods)
	}

	// Check for per-mode custom hotkeys before mode-specific handling.
	// Custom hotkeys use the same action syntax as top-level hotkeys.
	// Try the raw key first (preserves full modifier combos like "Cmd+Shift+G"
	// even when sticky modifiers are active), then the stripped key.
	if h.handleCustomHotkey(rawKey) {
		return
	}

	if rawKey != key {
		if h.handleCustomHotkey(key) {
			return
		}
	}

	h.handleModeSpecificKey(key)
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

// handleCustomHotkey checks if the key matches a custom_hotkeys binding for the
// current mode. If matched, it executes the action (IPC command or shell command)
// using the same logic as top-level hotkeys. Returns true if the key was consumed.
// Caller must hold h.mu.
func (h *Handler) handleCustomHotkey(key string) bool {
	if h.executeHotkeyAction == nil {
		return false
	}

	currentModeName := domain.ModeString(h.appState.CurrentMode())

	customHotkeys := h.config.CustomHotkeysForMode(currentModeName)
	if len(customHotkeys) == 0 {
		return false
	}

	normalizedKey := config.NormalizeKeyForComparison(key)

	// Phase 1: complete pending sequence if available and still valid.
	if h.customHotkeyLastKey != "" {
		pending := h.customHotkeyLastKey
		pendingAt := h.customHotkeyLastKeyTime
		h.customHotkeyLastKey = ""
		h.customHotkeyLastKeyTime = 0

		if pendingAt > 0 && time.Since(time.Unix(0, pendingAt)) <= customHotkeySequenceTimeout {
			if bindKey, actions, ok := findCustomHotkeyMatch(
				customHotkeys,
				pending+normalizedKey,
			); ok {
				h.dispatchCustomHotkeyActions(currentModeName, bindKey, key, actions)

				return true
			}
		}

		// Sequence failed to complete — drop the pending key (it was already
		// consumed as a sequence start) and fall through to process the
		// current key normally via Phase 2/3.  This matches the old scroll
		// keymap behavior where an incomplete sequence silently discards the
		// first key.
	}

	// Phase 2: direct single-key match.
	if bindKey, actions, ok := findCustomHotkeyMatch(customHotkeys, normalizedKey); ok {
		h.dispatchCustomHotkeyActions(currentModeName, bindKey, key, actions)

		return true
	}

	// Phase 3: start a new sequence for two-letter bindings.
	if isCustomHotkeySequenceStart(customHotkeys, normalizedKey) {
		h.customHotkeyLastKey = normalizedKey
		h.customHotkeyLastKeyTime = time.Now().UnixNano()

		return true
	}

	return false
}

func findCustomHotkeyMatch(
	customHotkeys map[string]config.StringOrStringArray,
	normalizedKey string,
) (string, []string, bool) {
	for bindKey, actions := range customHotkeys {
		if config.NormalizeKeyForComparison(bindKey) == normalizedKey {
			return bindKey, actions, true
		}
	}

	return "", nil, false
}

func isCustomHotkeySequenceStart(
	customHotkeys map[string]config.StringOrStringArray,
	normalizedKey string,
) bool {
	if len(normalizedKey) != 1 {
		return false
	}

	for bindKey := range customHotkeys {
		normalizedBindKey := config.NormalizeKeyForComparison(bindKey)
		if len(normalizedBindKey) == 2 &&
			config.IsAllLetters(normalizedBindKey) &&
			strings.HasPrefix(normalizedBindKey, normalizedKey) {
			return true
		}
	}

	return false
}

func (h *Handler) dispatchCustomHotkeyActions(
	modeName string,
	bindKey string,
	rawKey string,
	actions []string,
) {
	h.logger.Info("Custom hotkey matched",
		zap.String("mode", modeName),
		zap.String("bindKey", bindKey),
		zap.String("key", rawKey),
		zap.Strings("actions", actions))

	// Execute in a goroutine so the event tap callback returns quickly.
	// This also avoids a deadlock: executeHotkeyAction may call
	// ipcController.HandleCommand -> ActivateModeWithOptions which
	// acquires h.mu, but we already hold it.
	capturedKey := bindKey

	capturedActions := append([]string(nil), actions...)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("panic in custom hotkey handler",
					zap.Any("recover", r),
					zap.String("key", capturedKey))
			}
		}()

		for _, actionStr := range capturedActions {
			trimmedAction := strings.TrimSpace(actionStr)
			if trimmedAction == "" {
				continue
			}

			err := h.executeHotkeyAction(capturedKey, trimmedAction)
			if err != nil {
				h.logger.Error("Custom hotkey action failed",
					zap.String("key", capturedKey),
					zap.String("action", trimmedAction),
					zap.Error(err))
			}
		}
	}()
}
