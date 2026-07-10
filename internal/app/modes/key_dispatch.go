package modes

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	hotkeySequenceTimeout = 500 * time.Millisecond
)

const keyUpPrefix = "__keyup_"

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

	// Handle key-up events for held-key repeat suppression.
	// The eventtap emits modifier-free key names on key-up, so we compare
	// only the base key name (last segment after "+") case-insensitively.
	if releasedKey, ok := strings.CutPrefix(key, keyUpPrefix); ok {
		if h.heldRepeatingKey != "" {
			baseHeld := h.heldRepeatingKey
			if i := strings.LastIndex(baseHeld, "+"); i >= 0 {
				baseHeld = baseHeld[i+1:]
			}

			if strings.EqualFold(releasedKey, baseHeld) {
				h.stopHeldRepeatLocked()
			}
		}

		return
	}

	// Suppress macOS native key repeats when a custom held-key repeat is active.
	// The custom goroutine handles repeat dispatch at heldRepeatInterval.
	// heldRepeatingKey stores the sticky-stripped key, so we strip before comparing.
	if h.heldRepeatingKey != "" {
		suppressKey := key

		activeMods := h.stickyModifiers()
		if activeMods != 0 && !strings.HasPrefix(suppressKey, modifierTogglePrefix) {
			suppressKey = h.stripStickyModifiersFromKey(suppressKey, activeMods)
		}

		if suppressKey == h.heldRepeatingKey {
			return
		}
	}

	if h.appState.CurrentMode() == domain.ModeHints &&
		h.hints != nil && h.hints.Context != nil && h.hints.Context.SearchActive() {
		h.handleSearchInputKey(key)

		return
	}

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
	// (rawKey and stripped key) share the same snapshot. Only needed when the
	// active mode defines per-app hotkey overrides.
	var bundleID string
	if h.modeHasAppHotkeyOverrides(h.appState.CurrentMode()) {
		bundleID = h.focusedBundleID()
	}

	// Check for per-mode hotkeys before mode-specific handling.
	// If sticky modifiers were stripped, resolve bindings with the stripped key
	// only. Sticky modifiers are for the next action, not Neru's own navigation
	// keys; using rawKey here would make a sticky Ctrl turn "c" into "Ctrl+c".
	if rawKey != key {
		if actions, bindKey, ok := h.handleHotkey(key, bundleID); ok {
			if len(actions) > 0 {
				h.maybeStartHeldRepeatLocked(key, bindKey, actions)
			}

			return
		}
	} else if actions, bindKey, ok := h.handleHotkey(rawKey, bundleID); ok {
		if len(actions) > 0 {
			h.maybeStartHeldRepeatLocked(rawKey, bindKey, actions)
		}

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

// modeHasAppHotkeyOverrides reports whether the given mode defines any per-app
// hotkey overrides. That is the only situation requiring the focused app's
// bundle ID to be resolved in order to select the correct per-mode hotkey table.
func (h *Handler) modeHasAppHotkeyOverrides(mode domain.Mode) bool {
	switch mode {
	case domain.ModeHints:
		return h.config.Hints.HasAppHotkeyOverrides()
	case domain.ModeGrid:
		return h.config.Grid.HasAppHotkeyOverrides()
	case domain.ModeRecursiveGrid:
		return h.config.RecursiveGrid.HasAppHotkeyOverrides()
	case domain.ModeScroll:
		return h.config.Scroll.HasAppHotkeyOverrides()
	case domain.ModeIdle, domain.ModeMonitorSelect:
		return false
	default:
		return false
	}
}

// ModeHotkeyOverride returns the per-mode hotkey actions bound to key in the
// currently active navigation mode, with ok=true, when such a binding exists.
// It returns nil, false in idle mode or when the active mode does not bind key.
//
// Global (idle-scope) hotkeys are delivered through an always-on, per-hotkey
// event tap that is independent of the event tap used for per-mode hotkeys.
// When a key is bound both globally and by the active mode, the global tap
// consumes the event and runs the global action before the per-mode tap can
// act, so the per-mode binding for that key would otherwise never take effect.
// The global-hotkey dispatch path calls this so the more specific per-mode
// binding wins while a mode is active — e.g. a global "Primary+Ctrl+F" = "hints"
// launcher and a per-mode [hints.hotkeys] "Primary+Ctrl+F" = "recursive_grid"
// can coexist, with the latter applied whenever hints mode is active.
func (h *Handler) ModeHotkeyOverride(key string) ([]string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	mode := h.appState.CurrentMode()
	if mode == domain.ModeIdle {
		return nil, false
	}

	var bundleID string
	if h.modeHasAppHotkeyOverrides(mode) {
		bundleID = h.focusedBundleID()
	}

	hotkeys := h.config.HotkeysForModeAndApp(domain.ModeString(mode), bundleID)
	if len(hotkeys) == 0 {
		return nil, false
	}

	_, actions, ok := findHotkeyMatch(hotkeys, config.NormalizeKeyForComparison(key))
	if !ok {
		return nil, false
	}

	return actions, true
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
// using the same logic as top-level hotkeys. Returns the matched actions along
// with true if the key was consumed; returns nil, true for sequence starts (Phase 3)
// where no action is dispatched yet.
// Caller must hold h.mu. The bundleID is the focused app's bundle identifier,
// resolved once by the caller to avoid redundant accessibility IPC calls.
func (h *Handler) handleHotkey(key, bundleID string) (
	[]string, string, bool,
) {
	if h.executeHotkeyAction == nil {
		return nil, "", false
	}

	currentModeName := domain.ModeString(h.appState.CurrentMode())

	hotkeys := h.config.HotkeysForModeAndApp(currentModeName, bundleID)
	if len(hotkeys) == 0 {
		return nil, "", false
	}

	normalizedKey := config.NormalizeKeyForComparison(key)

	// Phase 1: complete pending sequence if available and still valid.
	if h.hotkeyLastKey != "" {
		pending := h.hotkeyLastKey
		pendingAt := h.hotkeyLastKeyTime
		h.hotkeyLastKey = ""
		h.hotkeyLastKeyTime = 0

		if pendingAt > 0 && time.Since(time.Unix(0, pendingAt)) <= hotkeySequenceTimeout {
			if bindKey, act, ok := findHotkeySequenceMatch(
				hotkeys,
				pending+normalizedKey,
			); ok {
				h.dispatchHotkeyActions(currentModeName, bindKey, key, act)

				return act, bindKey, true
			}
		}

		// Sequence failed to complete — drop the pending key (it was already
		// consumed as a sequence start) and fall through to process the
		// current key normally via Phase 2/3.  This matches the old scroll
		// keymap behavior where an incomplete sequence silently discards the
		// first key.
	}

	// Phase 2: direct single-key match.
	if bindKey, act, ok := findHotkeyMatch(hotkeys, normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, bindKey, key, act)

		return act, bindKey, true
	}

	// Phase 3: start a new sequence for two-letter bindings.
	if isHotkeySequenceStart(hotkeys, normalizedKey) {
		h.hotkeyLastKey = normalizedKey
		h.hotkeyLastKeyTime = time.Now().UnixNano()

		return nil, "", true
	}

	return nil, "", false
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
	h.logger.Debug("Hotkey matched",
		zap.String("mode", modeName),
		zap.String("bindKey", bindKey),
		zap.String("key", rawKey),
		zap.Int("action_count", len(actions)))

	// Note: we do NOT suppress modifiers here because this function is called
	// from handleHotkey which is called from HandleKeyPress while h.mu is held.
	// Suppression via SuppressModifiersForHotkey would deadlock. However, this
	// per-mode path is only reached when the per-mode event tap sees the key
	// event before the global hotkey tap consumes it. In the certain scenario
	// (switching modes on the same chord), the global hotkey tap consumes the
	// non-modifier key first, so this path is never hit for the mode-switch
	// hotkey. Modifier suppression for mode-switch actions is handled
	// synchronously in dispatchModeAwareHeldHotkey / dispatchModeAwareHotkeyAsync
	// in hotkeys.go before any async dispatch occurs.

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
				if derrors.IsCode(err, derrors.CodeChainBail) {
					return
				}

				h.logger.Error("Hotkey action failed",
					zap.String("key", capturedKey),
					zap.String("action", trimmedAction),
					zap.Error(err))
			}
		}
	}()
}

// maybeStartHeldRepeatLocked starts a custom repeat goroutine if the given
// actions are held-repeatable (scroll, page, mouse move) and held-key
// repeat is enabled in config.
// actions is the already-resolved action list from handleHotkey.
// bindKey is the normalised config-binding key (for consistent logging).
// Caller must hold h.mu.
func (h *Handler) maybeStartHeldRepeatLocked(key, bindKey string, actions []string) {
	if h.heldRepeatingCancel != nil {
		return
	}

	if !h.config.HeldRepeat.Enabled || !isHeldRepeatAction(actions) {
		return
	}

	h.startHeldRepeatLocked(key, bindKey, actions)
}

// startHeldRepeatLocked launches a goroutine that dispatches the held-key
// action at heldRepeatInterval until the key-up event arrives.
// bindKey is the normalised config-binding key (for consistent logging).
// Caller must hold h.mu.
func (h *Handler) startHeldRepeatLocked(key, bindKey string, actions []string) {
	cfg := h.config.HeldRepeat

	ctx, cancel := context.WithCancel(h.ctx)
	h.heldRepeatingKey = key
	h.heldRepeatingCancel = cancel

	capturedActions := append([]string(nil), actions...)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("panic in held repeat handler",
					zap.Any("recover", r),
					zap.String("key", bindKey))
			}
		}()

		initialTimer := time.NewTimer(time.Duration(cfg.InitialDelay) * time.Millisecond)
		defer initialTimer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-initialTimer.C:
		}

		ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, actionStr := range capturedActions {
					trimmedAction := strings.TrimSpace(actionStr)
					if trimmedAction == "" {
						continue
					}

					err := h.executeHotkeyAction(bindKey, trimmedAction)
					if err != nil {
						h.logger.Error("Held repeat action failed",
							zap.String("key", bindKey),
							zap.String("action", trimmedAction),
							zap.Error(err))
					}
				}
			}
		}
	}()
}

// isHeldRepeatAction reports whether the action list contains a single
// held-repeatable action (scroll, page, or relative mouse move).
func isHeldRepeatAction(actions []string) bool {
	if len(actions) != 1 {
		return false
	}

	parts := strings.SplitN(strings.TrimSpace(actions[0]), " ", 3) //nolint:mnd
	if len(parts) < 2 || parts[0] != "action" {
		return false
	}

	return action.IsHeldRepeatAction(action.Name(parts[1]))
}
