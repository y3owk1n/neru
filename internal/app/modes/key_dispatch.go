package modes

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/heldrepeat"
	"github.com/y3owk1n/neru/internal/app/sequence"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
	statepkg "github.com/y3owk1n/neru/internal/domain/state"
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
// to a held-repeat action would start a repeat that nothing ever stops. The
// synthetic release tears it down while letting the action fire once. The
// release is synthesized only when this press started the repeat, since an
// already-running repeat belongs to a physically held key. Press plus release
// run under one lock hold so no key event sees a transient repeat.
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

	// A fed key has no release to come, so it must never enter the glide.
	// It takes the discrete path and the synthesized release below.
	h.trackSelection(func() {
		h.fedPress = true
		h.handleKeyPress(key)
		h.fedPress = false

		// Only release a repeat this fed press just started. Leave any
		// pre-existing (physically held) repeat untouched.
		if repeatBefore == "" && h.heldRepeatingKey != "" {
			h.handleKeyPress(keyUpPrefix + base)
		}
	})
}

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.trackSelection(func() { h.handleKeyPress(key) })
}

// handleKeyPress contains the key-dispatch logic. The caller must hold
// h.mu. The whole body runs under the lock so held-repeat and modifier-toggle
// state stays consistent across the dispatch.
func (h *handlerState) handleKeyPress(key string) {
	// Handle key-up events for held-key repeat suppression.
	// The eventtap emits modifier-free key names on key-up, so we compare
	// only the base key name (last segment after "+") case-insensitively.
	if releasedKey, ok := strings.CutPrefix(key, keyUpPrefix); ok {
		h.motion.Release(motionKeyOf(releasedKey))

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
	// The same suppression covers a key steering a held-key glide.
	if h.heldRepeatingKey != "" || h.motion != nil {
		suppressKey := key

		activeMods := h.stickyModifiers()
		if activeMods != 0 && !strings.HasPrefix(suppressKey, modifierTogglePrefix) {
			suppressKey = h.stripStickyModifiersFromKey(suppressKey, activeMods)
		}

		if suppressKey == h.heldRepeatingKey {
			return
		}

		if h.motion.IsHeld(motionKeyOf(suppressKey)) {
			return
		}
	}

	if h.appState.CurrentMode() == domain.ModeHints &&
		h.hints != nil && h.hints.Context != nil && h.hints.Context.SearchActive() {
		h.handleSearchInputKey(key)

		return
	}

	// Cancel any pending modifier toggle when a non-modifier key is pressed.
	// For Shift+L, the modifier tap is canceled when L comes in.
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

	// A hint character held with a select_with modifier set types into the
	// label before any binding is asked. Validation refuses a binding that
	// such a chord would shadow. Sticky modifiers are already stripped, so only
	// physically held ones select an entry.
	if h.routeSelectWithLabelKey(key) {
		return
	}

	// Check for per-mode hotkeys before mode-specific handling.
	// If sticky modifiers were stripped, resolve bindings with the stripped key
	// only. Sticky modifiers are for the next action, not Neru's own navigation
	// keys. Using rawKey here would make a sticky Ctrl turn "c" into "Ctrl+c".
	if rawKey != key {
		if actions, bindKey, modeName, ok := h.handleHotkey(key); ok {
			if len(actions) > 0 {
				h.maybeStartHeldRepeat(key, bindKey, modeName, actions)
			}

			return
		}
	} else if actions, bindKey, modeName, ok := h.handleHotkey(rawKey); ok {
		if len(actions) > 0 {
			h.maybeStartHeldRepeat(rawKey, bindKey, modeName, actions)
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
// selects which bindings are in force, so a mode that binds none, or does not
// take per-app bindings at all, settles the same keymap whatever is focused,
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
// Global (idle-scope) hotkeys are delivered through an always-on hotkey
// event tap that is independent of the event tap used for per-mode hotkeys.
// When a key is bound both globally and by the active mode, the global tap
// consumes the event and runs the global action before the per-mode tap can
// act, so the per-mode binding for that key would otherwise never take effect.
// The global-hotkey dispatch path calls this so the more specific per-mode
// binding wins while a mode is active. For example, a global
// "Primary+Ctrl+F" = "hints" launcher and a per-mode [hints.hotkeys]
// "Primary+Ctrl+F" = "recursive_grid" can coexist, with the latter applied
// whenever hints mode is active.
func (h *Handler) ModeHotkeyOverride(key string) ([]string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() == domain.ModeIdle || h.vimTyping() {
		return nil, false
	}

	binding, ok := h.settledKeymap().Lookup(keyvocab.NormalizeKeyForComparison(key))
	if !ok {
		return nil, false
	}

	return binding.Steps, true
}

// vimTyping reports whether vim is typing into the app. Every key is then the
// app's, so a key bound in [vim.hotkeys] still types and only the global
// chords answer. Caller must hold h.mu.
func (h *handlerState) vimTyping() bool {
	return h.appState.CurrentMode() == domain.ModeVim && h.vim != nil && h.vim.typing
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
// command) using the same logic as top-level hotkeys. It returns the matched
// steps, the mode the binding was matched in, and true if the key was
// consumed, and nil steps with true for sequence starts (Phase 3), where
// nothing is dispatched yet. The mode is the one the steps were dispatched
// under, and the held-repeat slot reuses it so a repeat claims the press's
// slot rather than re-reading a mode a concurrent sequence may have moved.
//
// It consults the settled keymap and resolves nothing. Which bindings are in
// force was decided when the mode, the focused app or the configuration last
// changed, so nothing here can ask the operating system anything (ADR 0005).
//
// Caller must hold h.mu.
func (h *handlerState) handleHotkey(key string) (
	[]string, string, string, bool,
) {
	if h.executeActionSequence == nil {
		return nil, "", "", false
	}

	keymap, globalHotkeys := h.settledKeymaps()

	if h.vimTyping() {
		keymap = configpkg.Keymap{}
	}

	if keymap.Len() == 0 && globalHotkeys.Len() == 0 {
		return nil, "", "", false
	}

	currentModeName := domain.ModeString(h.appState.CurrentMode())
	normalizedKey := keyvocab.NormalizeKeyForComparison(key)

	// Phase 1: complete pending sequence if available and still valid.
	if h.hotkeyLastKey != "" {
		pending := h.hotkeyLastKey
		pendingAt := h.hotkeyLastKeyTime
		h.hotkeyLastKey = ""
		h.hotkeyLastKeyTime = 0

		if pendingAt > 0 && time.Since(time.Unix(0, pendingAt)) <= hotkeySequenceTimeout {
			if binding, ok := keymap.LookupSequence(pending + normalizedKey); ok {
				h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

				return binding.Steps, binding.Key, currentModeName, true
			}
		}

		// The sequence failed to complete. Drop the pending key (it was already
		// consumed as a sequence start) and fall through to process the
		// current key normally via Phase 2/3. This matches the old scroll
		// keymap behavior where an incomplete sequence silently discards the
		// first key.
	}

	// Phase 2: direct single-key match.
	if binding, ok := keymap.Lookup(normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

		return binding.Steps, binding.Key, currentModeName, true
	}

	// Phase 2b: the global [hotkeys] chord for the same key. The mode's own table
	// has already had its say above, so a mode that binds the chord keeps
	// winning it and this is reached only for one the mode leaves alone. There
	// the global binding is what the user still expects to work, and the
	// exclusive Linux capture leaves nothing else able to run it
	// (settledKeymaps).
	if binding, ok := globalHotkeys.Lookup(normalizedKey); ok {
		h.dispatchHotkeyActions(currentModeName, binding.Key, key, binding.Steps)

		return binding.Steps, binding.Key, currentModeName, true
	}

	// Phase 3: start a new sequence for two-letter bindings.
	if keymap.IsSequenceStart(normalizedKey) {
		h.hotkeyLastKey = normalizedKey
		h.hotkeyLastKeyTime = time.Now().UnixNano()

		return nil, "", "", true
	}

	return nil, "", "", false
}

func (h *handlerState) dispatchHotkeyActions(
	modeName string,
	bindKey string,
	rawKey string,
	actions []string,
) {
	// A gliding key moves the cursor for as long as it is down. Its
	// discrete step is never dispatched.
	if h.startMotion(rawKey, actions) {
		return
	}

	// One binding runs one sequence at a time, and one cursor slot is claimed
	// by one sequence at a time. A press that arrives while its binding's
	// previous run is still going is the kernel repeating the key, or a second
	// tap landing inside the run, and stacking it is what makes a pan drift:
	// the two runs share the cursor slot they save and restore through, and
	// their mouse-down and mouse-up steps interleave. Dropping the second one
	// rather than queueing it keeps a released key from leaving a tail of
	// sequences behind it. Different bindings never wait for each other, and
	// the same key bound in two modes is two bindings.
	//
	// Two bindings that share a cursor slot are in the same race even when
	// their keys differ: the pan macro's Shift+S and Shift+D both save and
	// restore through slot "pan", so a second pan pressed inside the first
	// run's sleeps restores a position the first run had already moved away
	// from. The slots a run touches are part of what it holds, read from the
	// steps the binding actually dispatches, macros expanded.
	//
	// A fed key is the exception, and it takes no slot at all. A script feeding
	// keys over IPC sends them back to back, far faster than any sequence
	// finishes, so the tail of a feed would be dropped as repeats that no one
	// repeated. It is a real press each time, and it runs.
	slot := newSequenceSlot(modeName, bindKey)

	claimed := !h.fedPress
	if claimed && !h.beginSequence(slot, sequenceClaims(actions, h.config.Macros)) {
		h.logger.Debug("Dropped a press of a binding still holding its resource",
			zap.String("mode", modeName),
			zap.String("bind_key", bindKey))

		return
	}

	h.logger.Debug("Hotkey matched",
		zap.String("mode", modeName),
		zap.String("bind_key", bindKey),
		zap.Int("action_count", len(actions)))

	// Note: we do NOT suppress modifiers here because this function is called
	// from handleHotkey which is called from HandleKeyPress while h.mu is held.
	// Suppression via SuppressModifiersForHotkey would deadlock. However, this
	// per-mode path is only reached when the per-mode event tap sees the key
	// event before the global hotkey tap consumes it. When switching modes on
	// the same chord, the global hotkey tap consumes the non-modifier key
	// first, so this path is never hit for the mode-switch hotkey. Modifier
	// suppression for mode-switch actions is handled synchronously in
	// dispatchModeAwareHeldHotkey / dispatchModeAwareHotkeyAsync in hotkeys.go
	// before any async dispatch occurs.

	// Execute in a goroutine so the event tap callback returns quickly.
	// This also avoids a deadlock. An action step may call
	// ipcController.HandleCommand -> ActivateMode which
	// acquires h.mu, but we already hold it.
	capturedActions := append([]string(nil), actions...)

	go h.runSequence(slot, bindKey, capturedActions, claimed)
}

// sequenceSlot identifies one binding for the single-flight guard: the mode the
// key was pressed in, then the key. Space in grid and Space in hints are two
// bindings, so a sequence that holds one of them for as long as a mode is up
// leaves the other answering.
//
// The separator is a NUL, which neither a mode name nor a normalised key
// contains, so two pairs never fold onto one slot.
type sequenceSlot string

// newSequenceSlot builds the slot for a press of bindKey in modeName.
func newSequenceSlot(modeName, bindKey string) sequenceSlot {
	return sequenceSlot(modeName + "\x00" + bindKey)
}

// sequenceClaim is one shared resource a sequence holds from dispatch to its
// last step: a cursor slot the steps save or restore through, named the way
// the step does, "slot\x00pan". Claims are global because the cursor slot
// store is, unlike the binding slot above, which is scoped to the mode the key
// was pressed in.
type sequenceClaim string

// cursorSlotClaimPrefix is what a cursor-slot claim starts with: the resource
// kind, then a NUL the way newSequenceSlot separates its parts, then the slot
// name. Today the cursor slot is the only claim kind, so the prefix is the
// whole key; another kind gets its own prefix and cannot fold onto these.
const cursorSlotClaimPrefix = "slot\x00"

// sequenceClaims returns what one dispatch of actions would claim. Today that
// is the cursor slots the steps name: every save_cursor_pos and
// restore_cursor_pos, the one it writes with --slot, the default slot for one
// without. A macro step is expanded first, so the claims are the body's, not
// the call's: "macro pan 0 200" claims what macros.pan saves and restores
// through. A nested macro is expanded too, once, depth-bounded by MaxDepth.
// A body that fails to expand contributes nothing, so a sequence never claims
// a resource a step could not have reached.
func sequenceClaims(
	actions []string,
	macros map[string]configpkg.StringOrStringArray,
) []sequenceClaim {
	var claims []sequenceClaim

	seen := make(map[sequenceClaim]struct{})

	claimSteps := func(steps []string) {
		for _, step := range steps {
			slot, ok := stepCursorSlot(step, macros, 0)
			if !ok {
				continue
			}

			claim := sequenceClaim(cursorSlotClaimPrefix + slot)
			if _, dup := seen[claim]; dup {
				continue
			}

			seen[claim] = struct{}{}

			claims = append(claims, claim)
		}
	}

	claimSteps(actions)

	return claims
}

// stepCursorSlot returns the cursor slot one step touches, when it is a
// save_cursor_pos or restore_cursor_pos, or a macro whose body contains one.
// depth is the macro nesting already expanded above it, bounded by the
// executor's own MaxDepth. Caller must hold h.mu.
func stepCursorSlot(
	step string,
	macros map[string]configpkg.StringOrStringArray,
	depth int,
) (string, bool) {
	if name, args, isMacro := configpkg.ParseMacroCall(step); isMacro {
		if depth >= sequence.MaxDepth {
			return "", false
		}

		body, defined := macros[name]
		if !defined {
			return "", false
		}

		for _, bodyStep := range configpkg.ExpandMacroSteps(body, args) {
			if slot, ok := stepCursorSlot(bodyStep, macros, depth+1); ok {
				return slot, true
			}
		}

		return "", false
	}

	parts := configpkg.SplitStepArgs(step)
	if len(parts) < 2 || parts[0] != action.PrefixAction {
		return "", false
	}

	name := action.Name(parts[1])
	if name != action.NameSaveCursorPos && name != action.NameRestoreCursorPos {
		return "", false
	}

	// The slot is --slot's value, in either spelling the step parser accepts.
	for idx := 2; idx < len(parts); idx++ {
		if value, ok := strings.CutPrefix(parts[idx], "--slot="); ok {
			return value, true
		}

		if parts[idx] == "--slot" && idx+1 < len(parts) {
			return parts[idx+1], true
		}
	}

	// A save or restore with no --slot is the default slot.
	return statepkg.DefaultCursorSlot, true
}

// runSequence runs one action sequence in its own goroutine and releases the
// binding's single-flight slot whichever way it ends, a panic in a step
// included. A panic is recovered here rather than left to take the daemon down,
// and the slot is released after it so the binding is not stuck for the rest of
// the session. Deferred order is what makes that work: the recover runs first,
// then the release.
//
// claimed says whether this run took the slot. A fed key's run holds none, so it
// releases nothing: that slot may belong to a press that is still running.
func (h *handlerState) runSequence(
	slot sequenceSlot,
	bindKey string,
	actions []string,
	claimed bool,
) {
	if claimed {
		defer h.endSequence(slot)
	}

	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in hotkey handler",
				zap.Any("recover", r),
				zap.String("bind_key", bindKey))
		}
	}()

	h.executeActionSequence(bindKey, actions, nil)
}

// beginSequence claims a binding's single-flight slot and every cursor slot its
// steps save or restore through, reporting whether it got them all. Claims are
// global rather than mode-scoped, because the cursor slot store is: slot "pan"
// in grid mode is the position slot "pan" in hints holds. The guard's mutex is
// the outer Handler's, a leaf below h.mu taken for two map writes, so this is
// safe both from dispatch, which holds h.mu, and from the held-repeat loop,
// which holds nothing.
func (h *handlerState) beginSequence(slot sequenceSlot, claims []sequenceClaim) bool {
	seqMu := &h.outer.seqMu

	seqMu.Lock()
	defer seqMu.Unlock()

	if h.seqInFlight == nil {
		h.seqInFlight = make(map[sequenceSlot]struct{})
	}

	for _, claim := range claims {
		if _, held := h.claimedSlots[claim]; held {
			return false
		}
	}

	if _, running := h.seqInFlight[slot]; running {
		return false
	}

	if h.claimedSlots == nil {
		h.claimedSlots = make(map[sequenceClaim]sequenceSlot)
	}

	// Every claim is free, so the run gets all of them. Nothing can take one
	// back between here and the release: dispatches only ever claim, and they
	// run the same check under the same lock.
	for _, claim := range claims {
		h.claimedSlots[claim] = slot
	}

	h.seqInFlight[slot] = struct{}{}

	return true
}

// endSequence releases the slot beginSequence claimed, and with it every cursor
// slot the run held. The sequence's own goroutine calls it, holding nothing
// else, and it reaches nothing but the two maps.
func (h *handlerState) endSequence(slot sequenceSlot) {
	seqMu := &h.outer.seqMu

	seqMu.Lock()
	defer seqMu.Unlock()

	for claim, owner := range h.claimedSlots {
		if owner == slot {
			delete(h.claimedSlots, claim)
		}
	}

	delete(h.seqInFlight, slot)
}

// maybeStartHeldRepeat starts a custom repeat goroutine if the given
// actions are held-repeatable (scroll, page, mouse move) and held-key
// repeat is enabled in config.
// actions is the already-resolved action list from handleHotkey.
// bindKey is the normalised config-binding key (for consistent logging).
// modeName is the mode handleHotkey matched the binding in.
// Caller must hold h.mu.
func (h *handlerState) maybeStartHeldRepeat(key, bindKey, modeName string, actions []string) {
	if h.heldRepeatingCancel != nil || h.motionApplies(actions) {
		return
	}

	if !h.config.HeldRepeat.Enabled || !isHeldRepeatAction(actions) {
		return
	}

	h.startHeldRepeat(key, bindKey, modeName, actions)
}

// startHeldRepeat launches a goroutine that dispatches the held-key
// action at heldRepeatInterval until the key-up event arrives.
// bindKey is the normalised config-binding key (for consistent logging).
// modeName is the mode handleHotkey matched the binding in.
// Caller must hold h.mu.
func (h *handlerState) startHeldRepeat(key, bindKey, modeName string, actions []string) {
	cfg := h.config.HeldRepeat

	ctx, cancel := context.WithCancel(h.ctx)
	h.heldRepeatingKey = key
	h.heldRepeatingCancel = cancel

	// The repeat claims the same slot the press claimed: modeName is the one
	// handleHotkey dispatched the press under, so the two agree by construction
	// and a concurrent sequence changing modes cannot slip between them. A held
	// key that starts a sequence changing modes keeps claiming its own slot,
	// rather than one no further tick would release.
	//
	// It holds the same cursor slots too. The steps are the press's own, so the
	// claims repeat tick for tick and one run of a pan is on screen at a time.
	slot := newSequenceSlot(modeName, bindKey)
	claims := sequenceClaims(actions, h.config.Macros)

	capturedActions := append([]string(nil), actions...)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in held repeat handler",
					zap.Any("recover", r),
					zap.String("bind_key", bindKey))
			}
		}()

		heldrepeat.Run(ctx, cfg, capturedActions, func(tickActions []string) {
			h.dispatchRepeatedSequence(slot, claims, bindKey, tickActions)
		})
	}()
}

// dispatchRepeatedSequence starts one repeat tick of a held binding. The tick
// goes through the same single-flight slot, and holds the same cursor slots, as
// the press that started the repeat, so a sequence slower than interval_ms sets
// the cadence: the ticks it is still running through are dropped, and the next
// one after it ends lands. Queueing them instead would leave a tail of them
// running after the key is let go.
func (h *handlerState) dispatchRepeatedSequence(
	slot sequenceSlot,
	claims []sequenceClaim,
	bindKey string,
	actions []string,
) {
	if !h.beginSequence(slot, claims) {
		h.logger.Debug("Dropped a repeat of a binding still holding its resource",
			zap.String("bind_key", bindKey))

		return
	}

	go h.runSequence(slot, bindKey, append([]string(nil), actions...), true)
}

// isHeldRepeatAction reports whether the action list contains a single
// held-repeatable action (scroll, page, relative mouse move, cell move or
// snap).
func isHeldRepeatAction(actions []string) bool {
	return action.IsHeldRepeatAction(action.Name(configpkg.HeldRepeatActionName(actions)))
}

// motionKeyOf reduces a key to the name its release event carries: the base
// segment, case-folded, since the event tap emits key-ups without modifiers.
func motionKeyOf(key string) string {
	if i := strings.LastIndex(key, "+"); i >= 0 {
		key = key[i+1:]
	}

	return strings.ToLower(key)
}

// motionApplies reports whether a held binding glides rather than repeats.
// Caller must hold h.mu.
func (h *handlerState) motionApplies(actions []string) bool {
	if h.motion == nil {
		return false
	}

	_, _, ok := h.config.HeldRepeat.HeldMotion(actions)

	return ok
}

// startMotion presses key into the glide's held set when its binding
// qualifies, reporting whether it did. Caller must hold h.mu. The controller's
// lock sits below it and never calls back.
func (h *handlerState) startMotion(key string, actions []string) bool {
	if h.motion == nil || h.fedPress {
		return false
	}

	dir, step, ok := h.config.HeldRepeat.HeldMotion(actions)
	if !ok {
		return false
	}

	// The discrete step would have gone through the IPC handler, which drops
	// the mode's selection so a later click lands where the cursor went. The
	// glide never gets there, so the selection is dropped here.
	if tracker, ok := activeModeExtension[selectionTracker](h); ok {
		tracker.ClearSelectionPoint()
	}

	h.motion.Press(motionKeyOf(key), dir, step)

	return true
}
