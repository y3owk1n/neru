package modes

import (
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const modifierTogglePrefix = "__modifier_"

// modifierToggleDebounce is the short quiet window after modifier release
// before we commit a sticky toggle. If a regular key arrives in this window,
// the tap is treated as part of a combo instead of a sticky toggle.
const (
	modifierToggleDebounce = 50 * time.Millisecond
	// Keep launch-hotkey modifiers suppressed long enough that users can
	// comfortably hold Cmd/Shift after entering a mode without their eventual
	// release being reinterpreted as a fresh sticky-modifier tap.
	activationModifierSuppressionWindow = 2 * time.Second
)

var modifierToggleMap = map[string]action.Modifiers{
	keyPartCmd:   action.ModCmd,
	keyPartShift: action.ModShift,
	keyPartAlt:   action.ModAlt,
	keyPartCtrl:  action.ModCtrl,
}

var allStickyModifiers = []action.Modifiers{
	action.ModCmd,
	action.ModShift,
	action.ModAlt,
	action.ModCtrl,
}

// parseModifierEvent parses a modifier event key like "__modifier_shift_down"
// or "__modifier_cmd_up" into the modifier and whether it's a down or up event.
// Returns (modifier, isDown, ok).
func parseModifierEvent(key string) (action.Modifiers, bool, bool) {
	if !strings.HasPrefix(key, modifierTogglePrefix) {
		return 0, false, false
	}

	suffix := strings.ToLower(strings.TrimPrefix(key, modifierTogglePrefix))

	if before, ok := strings.CutSuffix(suffix, "_down"); ok {
		name := before
		mod, ok := modifierToggleMap[name]

		return mod, true, ok
	}

	if before, ok := strings.CutSuffix(suffix, "_up"); ok {
		name := before
		mod, ok := modifierToggleMap[name]

		return mod, false, ok
	}

	return 0, false, false
}

// handleModifierToggle processes modifier down/up events for sticky toggle.
// A modifier becomes sticky when its down/up pair completes without any
// intervening regular key, and pressing the same modifier again toggles it off.
func (h *Handler) handleModifierToggle(key string) bool {
	if !h.stickyModifiersEnabled() {
		return false
	}

	h.expireSuppressedModifiersIfNeeded()

	mod, isDown, ok := parseModifierEvent(key)
	if !ok {
		return false
	}

	if isDown {
		if h.suppressedModifiers.Has(mod) {
			// Record this as a fresh press so the UP handler can
			// distinguish between a chord release (no fresh press)
			// and a non-chord modifier tap (fresh press while suppressed).
			if h.modifierFreshPress == nil {
				h.modifierFreshPress = make(map[action.Modifiers]bool)
			}

			h.modifierFreshPress[mod] = true
			h.suppressedModifiers &^= mod
			h.usedInChordModifiers &^= mod
			h.heldModifiers |= mod
			delete(h.pendingModifierKeys, mod)
			h.stopPendingModifierTimer(mod)

			return true
		}

		// Non-chord modifier detection at Go level: if a non-suppressed modifier
		// is pressed while suppressedModifiers is set (from a previous
		// hotkey chord), this signals a non-chord modifier press. Create pending keys
		// for all suppressed modifiers so their subsequent UP events
		// are handled as sticky toggles.
		if h.suppressedModifiers != 0 {
			now := time.Now()

			for _, suppressedMod := range allStickyModifiers {
				if h.suppressedModifiers.Has(suppressedMod) {
					if h.pendingModifierKeys == nil {
						h.pendingModifierKeys = make(map[action.Modifiers]time.Time)
					}

					h.pendingModifierKeys[suppressedMod] = now
				}
			}

			h.suppressedModifiers = 0
			h.usedInChordModifiers = 0
			h.suppressedUntil = time.Time{}
			h.logger.Debug("Suppression cleared by non-chord modifier press, pending keys created")
		}

		if h.pendingModifierKeys == nil {
			h.pendingModifierKeys = make(map[action.Modifiers]time.Time)
		}

		h.heldModifiers |= mod
		h.usedInChordModifiers &^= mod
		h.stopPendingModifierTimer(mod)
		h.pendingModifierKeys[mod] = time.Now()
		h.logger.Debug("Modifier key down", zap.String("key", strings.ToLower(key)))

		return true
	}

	h.heldModifiers &^= mod

	if h.suppressedModifiers.Has(mod) {
		delete(h.pendingModifierKeys, mod)
		h.stopPendingModifierTimer(mod)
		h.usedInChordModifiers &^= mod
		h.suppressedModifiers &^= mod
		h.logger.Debug("Modifier key up ignored (suppressed activation modifier)",
			zap.String("key", key),
			zap.String("modifier", mod.String()))

		return true
	}

	downTime, pending := h.pendingModifierKeys[mod]
	if !pending {
		// If this modifier was pressed fresh while suppressed (set by
		// the suppressed DOWN handler), treat the release as a tap.
		// This handles the non-chord modifier case where stale suppression bits
		// would otherwise cause the UP to be silently dropped.
		if h.modifierFreshPress[mod] {
			delete(h.modifierFreshPress, mod)

			now := time.Now()

			if h.pendingModifierKeys == nil {
				h.pendingModifierKeys = make(map[action.Modifiers]time.Time)
			}

			h.pendingModifierKeys[mod] = now
			h.scheduleModifierToggle(mod, now)
			h.logger.Debug("Modifier key up (fresh press while suppressed, treating as tap)",
				zap.String("key", key),
				zap.String("modifier", mod.String()))

			return true
		}

		h.logger.Debug("Modifier key up ignored (no matching pending down)",
			zap.String("key", key),
			zap.Any("pending", h.pendingModifierKeys))
		h.usedInChordModifiers &^= mod

		return true
	}

	if h.usedInChordModifiers.Has(mod) {
		delete(h.pendingModifierKeys, mod)
		h.stopPendingModifierTimer(mod)
		h.usedInChordModifiers &^= mod
		h.logger.Debug("Modifier key up ignored (modifier was used in chord)",
			zap.String("key", key),
			zap.String("modifier", mod.String()))

		return true
	}

	// If a tap-max-duration is configured, reject holds that exceeded it
	// before even scheduling the debounce.
	if maxDur := h.config.StickyModifiers.TapMaxDuration; maxDur > 0 {
		elapsed := time.Since(downTime)
		if elapsed > time.Duration(maxDur)*time.Millisecond {
			delete(h.pendingModifierKeys, mod)
			h.logger.Debug("Modifier tap rejected (held too long)",
				zap.String("modifier", mod.String()),
				zap.Duration("held", elapsed),
				zap.Int("maxMs", maxDur))

			return true
		}
	}

	// Wait briefly before committing the toggle so remapped follow-up keys can
	// still cancel the tap and turn it into a normal modifier combo.
	h.scheduleModifierToggle(mod, downTime)

	return true
}

func (h *Handler) stopPendingModifierTimer(mod action.Modifiers) {
	if h.pendingModifierTimers == nil {
		return
	}

	if existingTimer, exists := h.pendingModifierTimers[mod]; exists {
		existingTimer.Stop()
		delete(h.pendingModifierTimers, mod)
	}
}

// scheduleModifierToggle starts a debounce timer that will toggle the given
// modifier after modifierToggleDebounce unless canceled by a regular key press.
func (h *Handler) scheduleModifierToggle(mod action.Modifiers, downTime time.Time) {
	if h.pendingModifierTimers == nil {
		h.pendingModifierTimers = make(map[action.Modifiers]*time.Timer)
	}

	h.stopPendingModifierTimer(mod)

	timerSession := h.modeSession

	h.logger.Debug("Scheduling modifier toggle debounce",
		zap.String("modifier", mod.String()),
		zap.Duration("delay", modifierToggleDebounce))

	timer := time.AfterFunc(modifierToggleDebounce, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		defer h.notifyDebounceComplete()

		// Guard against stale timer: if the mode session changed (user exited
		// and re-entered a mode) while we were waiting, this timer belongs to
		// a previous session and must not toggle anything. The primary cleanup
		// path (cancelPendingModifierToggle via setAppModeLocked) already
		// stops timers and nils pendingModifierKeys, but this check provides
		// defense-in-depth in case a timer fires between the mode exit and the
		// cancel — matching the pattern used by refreshHintsTimer.
		if h.modeSession != timerSession {
			delete(h.pendingModifierTimers, mod)

			return
		}

		pendingDownTime, stillPending := h.pendingModifierKeys[mod]
		if !stillPending {
			h.logger.Debug("Modifier toggle debounce canceled (regular key intervened)",
				zap.String("modifier", mod.String()))

			delete(h.pendingModifierTimers, mod)

			return
		}

		if !pendingDownTime.Equal(downTime) {
			h.logger.Debug("Modifier toggle debounce skipped (stale timer from rapid double-tap)",
				zap.String("modifier", mod.String()))
			delete(h.pendingModifierTimers, mod)

			return
		}

		delete(h.pendingModifierKeys, mod)
		delete(h.pendingModifierTimers, mod)
		newModifiers := h.modifierState.Toggle(mod)
		isDownNow := newModifiers.Has(mod)

		modName := ""
		switch mod {
		case action.ModCmd:
			modName = keyPartCmd
		case action.ModShift:
			modName = keyPartShift
		case action.ModAlt:
			modName = keyPartAlt
		case action.ModCtrl:
			modName = keyPartCtrl
		}

		if modName != "" && h.postModifierEvent != nil {
			h.postModifierEvent(modName, isDownNow)
		}

		h.logger.Debug("Sticky modifier toggled (after debounce)",
			zap.String("modifier", mod.String()),
			zap.String("state", newModifiers.String()))
	})

	h.pendingModifierTimers[mod] = timer
}

// notifyDebounceComplete sends a non-blocking signal on debounceNotify so
// tests can synchronize with the timer callback. In production the channel
// is nil and this is a no-op.
func (h *Handler) notifyDebounceComplete() {
	if h.debounceNotify != nil {
		select {
		case h.debounceNotify <- struct{}{}:
		default:
		}
	}
}

// clearStickyModifiers releases any physically held sticky modifiers and resets internal state.
// UsedInChordModifiers is NOT reset here because it may have been set by
// SuppressModifiersForHotkey (called from the global hotkey dispatch path)
// to prevent modifier UP events from starting debounce timers during a
// mode switch. Resetting it here would undo the suppression before the
// UP events arrive, causing unintended sticky modifier toggles.
func (h *Handler) clearStickyModifiers() {
	if h.modifierState == nil {
		return
	}

	mods := h.modifierState.Current()
	if h.postModifierEvent != nil {
		if mods.Has(action.ModCmd) {
			h.postModifierEvent(keyPartCmd, false)
		}

		if mods.Has(action.ModShift) {
			h.postModifierEvent(keyPartShift, false)
		}

		if mods.Has(action.ModAlt) {
			h.postModifierEvent(keyPartAlt, false)
		}

		if mods.Has(action.ModCtrl) {
			h.postModifierEvent(keyPartCtrl, false)
		}
	}

	h.modifierState.Reset()
	h.heldModifiers = 0
}

func (h *Handler) cancelPendingModifierToggle() {
	if len(h.pendingModifierKeys) > 0 {
		h.pendingModifierKeys = nil
		h.logger.Debug("Modifier tap canceled")
	}

	// Stop all debounce timers so the toggle callback finds no pending entry.
	for key, timer := range h.pendingModifierTimers {
		timer.Stop()
		delete(h.pendingModifierTimers, key)
	}
}

func (h *Handler) markHeldModifiersUsedInChord() {
	h.expireSuppressedModifiersIfNeeded()

	for _, mod := range allStickyModifiers {
		if h.heldModifiers.Has(mod) {
			h.usedInChordModifiers |= mod
		}
	}
}

// SuppressModifiersUntilReleased marks the given modifiers as temporarily
// ineligible for sticky toggle. Suppression ends on the first matching release
// or after a short timeout if that release never arrives.
func (h *Handler) SuppressModifiersUntilReleased(mods action.Modifiers) {
	if mods == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.suppressedModifiers |= mods
	h.suppressedUntil = time.Now().Add(activationModifierSuppressionWindow)

	for _, mod := range allStickyModifiers {
		if mods.Has(mod) {
			delete(h.pendingModifierKeys, mod)
			h.stopPendingModifierTimer(mod)
			h.usedInChordModifiers &^= mod
		}
	}
}

// SuppressModifiersForHotkey suppresses the given modifiers from sticky toggle
// and also marks them as used-in-chord so that any modifier UP events that were
// already enqueued by the per-mode event tap (before the synchronous suppression
// could take effect) will be ignored by handleModifierToggle instead of starting
// a debounce timer. This provides defense-in-depth against the race between the
// per-mode event tap thread and the global hotkey callback goroutine.
func (h *Handler) SuppressModifiersForHotkey(mods action.Modifiers) {
	if mods == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.suppressedModifiers |= mods
	h.suppressedUntil = time.Now().Add(activationModifierSuppressionWindow)

	for _, mod := range allStickyModifiers {
		if mods.Has(mod) {
			// Clean up any modifierFreshPress state from a previous suppressed
			// DOWN handler invocation — this prevents stale fresh-press flags
			// from triggering spurious sticky toggles on the next cycle.
			delete(h.modifierFreshPress, mod)
			delete(h.pendingModifierKeys, mod)
			h.stopPendingModifierTimer(mod)

			// Mark as used-in-chord so that modifier UP events from the per-mode
			// event tap that arrive before our suppression takes effect are still
			// caught by the usedInChordModifiers check in handleModifierToggle.
			h.usedInChordModifiers |= mod
		}
	}
}

func (h *Handler) expireSuppressedModifiersIfNeeded() {
	if h.suppressedModifiers == 0 || h.suppressedUntil.IsZero() {
		return
	}

	if time.Now().Before(h.suppressedUntil) {
		return
	}

	h.suppressedModifiers = 0
	h.suppressedUntil = time.Time{}
	h.usedInChordModifiers = 0
	h.modifierFreshPress = nil
}

func (h *Handler) stickyModifiersEnabled() bool {
	if h.config == nil {
		return false
	}

	return h.config.StickyModifiers.Enabled
}

func (h *Handler) stickyModifiers() action.Modifiers {
	if h.modifierState == nil {
		return 0
	}

	return h.modifierState.Current()
}

// StickyModifiers returns the currently active sticky modifiers.
func (h *Handler) StickyModifiers() action.Modifiers {
	return h.stickyModifiers()
}

func (h *Handler) drawStickyModifiersIndicator(xCoordinate, yCoordinate int) {
	if h.stickyIndicatorService == nil {
		return
	}

	if !h.stickyModifiersEnabled() {
		return
	}

	mods := h.stickyModifiers()
	if mods == 0 {
		return
	}

	symbols := stickyindicator.ModifierSymbolsString(mods)
	if symbols == "" {
		return
	}

	h.stickyIndicatorService.UpdateIndicatorPosition(xCoordinate, yCoordinate, symbols)
}
