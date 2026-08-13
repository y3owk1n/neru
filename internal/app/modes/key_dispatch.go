package modes

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/heldrepeat"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	hotkeySequenceTimeout = 500 * time.Millisecond
)

// keyUpPrefix aliases the shared wire vocabulary the event taps emit.
const keyUpPrefix = keyvocab.KeyUpPrefix

const (
	keyPartCmd    = "cmd"
	keyPartShift  = "shift"
	keyPartAlt    = "alt"
	keyPartCtrl   = "ctrl"
	keyPartOption = "option"
)

// HandleFedKeyPress dispatches a key injected over IPC as a discrete
// press-and-release. A fed key has no eventtap key-up companion, so one bound
// to a held-repeat action would start a repeat that nothing ever stops; the
// synthetic release tears it down while letting the action fire once. The
// release is synthesized only when this press started the repeat — an
// already-running repeat belongs to a physically held key — and press plus
// release run under one lock hold so no key event sees a transient repeat.
func (h *Handler) HandleFedKeyPress(key string) {
	// The key-up handler compares against the modifier-free base key, so strip
	// any modifier prefix before synthesizing the release.
	base := key
	if i := strings.LastIndex(base, "+"); i >= 0 {
		base = base[i+1:]
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	repeatBefore := h.heldRepeatingKey
	h.handleKeyPress(key)

	// Only release a repeat this fed press just started; leave any pre-existing
	// (physically held) repeat untouched.
	if repeatBefore == "" && h.heldRepeatingKey != "" {
		h.handleKeyPress(keyUpPrefix + base)
	}
}

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handleKeyPress(key)
}

// handleKeyPress contains the key-dispatch logic. The caller must hold
// h.mu; the whole body runs under the lock so held-repeat and modifier-toggle
// state stays consistent across the dispatch.
func (h *handlerState) handleKeyPress(key string) {
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
				h.stopHeldRepeat()
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

	// Check for per-mode hotkeys before mode-specific handling.
	// If sticky modifiers were stripped, resolve bindings with the stripped key
	// only. Sticky modifiers are for the next action, not Neru's own navigation
	// keys; using rawKey here would make a sticky Ctrl turn "c" into "Ctrl+c".
	if rawKey != key {
		if actions, bindKey, ok := h.handleHotkey(key); ok {
			if len(actions) > 0 {
				h.maybeStartHeldRepeat(key, bindKey, actions)
			}

			return
		}
	} else if actions, bindKey, ok := h.handleHotkey(rawKey); ok {
		if len(actions) > 0 {
			h.maybeStartHeldRepeat(rawKey, bindKey, actions)
		}

		return
	}

	h.handleModeSpecificKey(key)
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *handlerState) handleModeSpecificKey(key string) {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.HandleKey(key)
}

// activeModeHasAppHotkeyOverrides reports whether the active mode defines any
// per-app hotkey overrides. That is the only situation in which the focused app
// selects which bindings are in force, so a mode that binds none — or does not
// take per-app bindings at all — settles the same keymap whatever is focused,
// and never has to learn which application that is.
func (h *handlerState) activeModeHasAppHotkeyOverrides() bool {
	reporter, ok := activeModeExtension[hotkeyOverrideReporter](h)
	if !ok {
		return false
	}

	return reporter.HasAppHotkeyOverrides()
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

	if h.appState.CurrentMode() == domain.ModeIdle {
		return nil, false
	}

	binding, ok := h.settledKeymap().Lookup(config.NormalizeKeyForComparison(key))
	if !ok {
		return nil, false
	}

	return binding.Steps, true
}

// stripStickyModifiersFromKey removes any currently active sticky modifiers from the
// incoming key string so that physical injections don't break expected key bindings.
func (h *handlerState) stripStickyModifiersFromKey(key string, mods action.Modifiers) string {
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

// handleHotkey checks whether the key matches a binding in the keymap in
// force. If it does, it executes the binding's steps (IPC command or shell
// command) using the same logic as top-level hotkeys. Returns the matched steps
// along with true if the key was consumed; returns nil, true for sequence
// starts (Phase 3) where nothing is dispatched yet.
//
// It consults the settled keymap and resolves nothing: which bindings are in
// force was decided when the mode, the focused app or the configuration last
// changed, so nothing here can ask the operating system anything (ADR 0005).
//
// Caller must hold h.mu.
func (h *handlerState) handleHotkey(key string) (
	[]string, string, bool,
) {
	if h.executeActionSequence == nil {
		return nil, "", false
	}

	keymap, globalHotkeys := h.settledKeymaps()
	if keymap.Len() == 0 && globalHotkeys.Len() == 0 {
		return nil, "", false
	}

	currentModeName := domain.ModeString(h.appState.CurrentMode())
	normalizedKey := config.NormalizeKeyForComparison(key)

	// Phase 1: complete pending sequence if available and still valid.
	if h.hotkeyLastKey != "" {
		pending := h.hotkeyLastKey
		pendingAt := h.hotkeyLastKeyTime
		h.hotkeyLastKey = ""
		h.hotkeyLastKeyTime = 0

		if pendingAt > 0 && time.Since(time.Unix(0, pendingAt)) <= hotkeySequenceTimeout {
			if binding, ok := keymap.LookupSequence(pending + normalizedKey); ok {
				h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

				return binding.Steps, binding.Key, true
			}
		}

		// Sequence failed to complete — drop the pending key (it was already
		// consumed as a sequence start) and fall through to process the
		// current key normally via Phase 2/3.  This matches the old scroll
		// keymap behavior where an incomplete sequence silently discards the
		// first key.
	}

	// Phase 2: direct single-key match.
	if binding, ok := keymap.Lookup(normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

		return binding.Steps, binding.Key, true
	}

	// Phase 2b: the global [hotkeys] chord for the same key. The mode's own table
	// has already had its say above, so a mode that binds the chord keeps
	// winning it and this is reached only for one the mode leaves alone — where
	// the global binding is what the user still expects to work, and where the
	// exclusive Linux capture leaves nothing else able to run it
	// (settledKeymaps).
	if binding, ok := globalHotkeys.Lookup(normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

		return binding.Steps, binding.Key, true
	}

	// Phase 3: start a new sequence for two-letter bindings.
	if keymap.IsSequenceStart(normalizedKey) {
		h.hotkeyLastKey = normalizedKey
		h.hotkeyLastKeyTime = time.Now().UnixNano()

		return nil, "", true
	}

	return nil, "", false
}

func (h *handlerState) dispatchHotkeyActions(
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
	// This also avoids a deadlock: an action step may call
	// ipcController.HandleCommand -> ActivateMode which
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

		h.executeActionSequence(capturedKey, capturedActions)
	}()
}

// maybeStartHeldRepeat starts a custom repeat goroutine if the given
// actions are held-repeatable (scroll, page, mouse move) and held-key
// repeat is enabled in config.
// actions is the already-resolved action list from handleHotkey.
// bindKey is the normalised config-binding key (for consistent logging).
// Caller must hold h.mu.
func (h *handlerState) maybeStartHeldRepeat(key, bindKey string, actions []string) {
	if h.heldRepeatingCancel != nil {
		return
	}

	if !h.config.HeldRepeat.Enabled || !isHeldRepeatAction(actions) {
		return
	}

	h.startHeldRepeat(key, bindKey, actions)
}

// startHeldRepeat launches a goroutine that dispatches the held-key
// action at heldRepeatInterval until the key-up event arrives.
// bindKey is the normalised config-binding key (for consistent logging).
// Caller must hold h.mu.
func (h *handlerState) startHeldRepeat(key, bindKey string, actions []string) {
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

		heldrepeat.Run(ctx, cfg, capturedActions, func(tickActions []string) {
			h.executeActionSequence(bindKey, tickActions)
		})
	}()
}

// isHeldRepeatAction reports whether the action list contains a single
// held-repeatable action (scroll, page, relative mouse move, or cell move).
func isHeldRepeatAction(actions []string) bool {
	return action.IsHeldRepeatAction(action.Name(config.HeldRepeatActionName(actions)))
}
