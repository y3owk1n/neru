package modes

import (
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const hotkeySequenceTimeout = 500 * time.Millisecond

const (
	keyPartCmd    = "cmd"
	keyPartShift  = "shift"
	keyPartAlt    = "alt"
	keyPartCtrl   = "ctrl"
	keyPartOption = "option"
)

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Cancel any pending modifier toggle if a non-modifier key is pressed
	// This handles the case where Shift+L is pressed - the modifier tap
	// is canceled when L comes in
	if !strings.HasPrefix(key, modifierTogglePrefix) {
		h.markHeldModifiersUsedInChord()
		h.cancelPendingModifierToggle()
	}

	// Check for modifier toggle keys before any other processing
	if h.handleModifierToggle(key) {
		return
	}

	// Save the raw key before sticky modifier stripping so we can try
	// hotkey matching with the original modifier combo later.
	rawKey := key

	// Sticky modifiers are also physically posted into macOS so apps can react
	// as if the key is held. Strip those sticky prefixes back out for Neru's own
	// binding resolution so regular mode keys still behave predictably.
	activeMods := h.stickyModifiers()
	if activeMods != 0 && !strings.HasPrefix(key, modifierTogglePrefix) {
		key = h.stripStickyModifiersFromKey(key, activeMods)
	}

	// Resolve the focused app bundle ID once so that both handleHotkey calls
	// (rawKey and stripped key) share the same snapshot.
	var bundleID string
	if h.appState.CurrentMode() == domain.ModeHints && h.config.Hints.HasAppHotkeyOverrides() {
		bundleID = h.focusedBundleID()
	}

	// Check for per-mode hotkeys before mode-specific handling.
	// If sticky modifiers were stripped, resolve bindings with the stripped key
	// only. Sticky modifiers are for the next action, not Neru's own navigation
	// keys; using rawKey here would make a sticky Ctrl turn "c" into "Ctrl+c".
	if rawKey != key {
		if h.handleHotkey(key, bundleID) {
			return
		}
	} else if h.handleHotkey(rawKey, bundleID) {
		return
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
		if i < len(parts)-1 {
			lowerPart := strings.ToLower(part)

			if lowerPart == keyPartCmd && mods.Has(action.ModCmd) {
				continue
			}

			if lowerPart == keyPartShift && mods.Has(action.ModShift) {
				continue
			}

			if lowerPart == keyPartAlt && mods.Has(action.ModAlt) {
				continue
			}

			if lowerPart == keyPartCtrl && mods.Has(action.ModCtrl) {
				continue
			}

			if lowerPart == keyPartOption && mods.Has(action.ModAlt) {
				continue
			}
		}

		newParts = append(newParts, part)
	}

	return strings.Join(newParts, "+")
}

// handleHotkey checks if the key matches a hotkeys binding for the
// current mode. If matched, it executes the action (IPC command or shell command)
// using the same logic as top-level hotkeys. Returns true if the key was consumed.
// Caller must hold h.mu. The bundleID is the focused app's bundle identifier,
// resolved once by the caller to avoid redundant accessibility IPC calls.
func (h *Handler) handleHotkey(key, bundleID string) bool {
	if h.executeHotkeyAction == nil {
		return false
	}

	currentModeName := domain.ModeString(h.appState.CurrentMode())

	hotkeys := h.config.HotkeysForModeAndApp(currentModeName, bundleID)
	if len(hotkeys) == 0 {
		return false
	}

	normalizedKey := config.NormalizeKeyForComparison(key)

	// Phase 1: complete pending sequence if available and still valid.
	if h.hotkeyLastKey != "" {
		pending := h.hotkeyLastKey
		pendingAt := h.hotkeyLastKeyTime
		h.hotkeyLastKey = ""
		h.hotkeyLastKeyTime = 0

		if pendingAt > 0 && time.Since(time.Unix(0, pendingAt)) <= hotkeySequenceTimeout {
			if bindKey, actions, ok := findHotkeySequenceMatch(
				hotkeys,
				pending+normalizedKey,
			); ok {
				h.dispatchHotkeyActions(currentModeName, bindKey, key, actions)

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
	if bindKey, actions, ok := findHotkeyMatch(hotkeys, normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, bindKey, key, actions)

		return true
	}

	// Phase 3: start a new sequence for two-letter bindings.
	if isHotkeySequenceStart(hotkeys, normalizedKey) {
		h.hotkeyLastKey = normalizedKey
		h.hotkeyLastKeyTime = time.Now().UnixNano()

		return true
	}

	return false
}

func findHotkeyMatch(
	hotkeys map[string]config.StringOrStringArray,
	normalizedKey string,
) (string, []string, bool) {
	for bindKey, actions := range hotkeys {
		normalizedBindKey := config.NormalizeKeyForComparison(bindKey)
		if normalizedBindKey == normalizedKey {
			return bindKey, actions, true
		}
	}

	return "", nil, false
}

// findHotkeySequenceMatch is like findHotkeyMatch but skips named keys
// (e.g. "Up", "F1"). It is used exclusively by Phase 1 (sequence completion)
// to prevent a concatenated sequence like "u"+"p" from matching the named key
// "Up" whose normalized form is also "up".
func findHotkeySequenceMatch(
	hotkeys map[string]config.StringOrStringArray,
	normalizedKey string,
) (string, []string, bool) {
	for bindKey, actions := range hotkeys {
		if config.IsValidNamedKey(bindKey) {
			continue
		}

		if config.NormalizeKeyForComparison(bindKey) == normalizedKey {
			return bindKey, actions, true
		}
	}

	return "", nil, false
}

func isHotkeySequenceStart(
	hotkeys map[string]config.StringOrStringArray,
	normalizedKey string,
) bool {
	if len(normalizedKey) != 1 {
		return false
	}

	for bindKey := range hotkeys {
		// Only consider genuine two-letter sequences (e.g. "gg"), not named
		// keys that happen to be two letters (e.g. "Up" normalizes to "up").
		if config.IsValidNamedKey(bindKey) {
			continue
		}

		normalizedBindKey := config.NormalizeKeyForComparison(bindKey)
		if len(normalizedBindKey) == 2 &&
			config.IsAllLetters(normalizedBindKey) &&
			strings.HasPrefix(normalizedBindKey, normalizedKey) {
			return true
		}
	}

	return false
}

func (h *Handler) dispatchHotkeyActions(
	modeName string,
	bindKey string,
	rawKey string,
	actions []string,
) {
	h.logger.Info("Hotkey matched",
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
				h.logger.Error("panic in hotkey handler",
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
				h.logger.Error("Hotkey action failed",
					zap.String("key", capturedKey),
					zap.String("action", trimmedAction),
					zap.Error(err))
			}
		}
	}()
}
