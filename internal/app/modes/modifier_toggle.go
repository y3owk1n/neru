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
	modifierToggleDebounce              = 50 * time.Millisecond
	activationModifierSuppressionWindow = 300 * time.Millisecond
)

var modifierToggleMap = map[string]action.Modifiers{
	"cmd":   action.ModCmd,
	"shift": action.ModShift,
	"alt":   action.ModAlt,
	"ctrl":  action.ModCtrl,
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
			h.heldModifiers |= mod
			delete(h.pendingModifierKeys, mod)
			h.stopPendingModifierTimer(mod)
			h.usedInChordModifiers &^= mod

			return true
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
			modName = "cmd"
		case action.ModShift:
			modName = "shift"
		case action.ModAlt:
			modName = "alt"
		case action.ModCtrl:
			modName = "ctrl"
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
func (h *Handler) clearStickyModifiers() {
	if h.modifierState == nil {
		return
	}

	mods := h.modifierState.Current()
	if h.postModifierEvent != nil {
		if mods.Has(action.ModCmd) {
			h.postModifierEvent("cmd", false)
		}

		if mods.Has(action.ModShift) {
			h.postModifierEvent("shift", false)
		}

		if mods.Has(action.ModAlt) {
			h.postModifierEvent("alt", false)
		}

		if mods.Has(action.ModCtrl) {
			h.postModifierEvent("ctrl", false)
		}
	}

	h.modifierState.Reset()
	h.heldModifiers = 0
	h.usedInChordModifiers = 0
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

func (h *Handler) expireSuppressedModifiersIfNeeded() {
	if h.suppressedModifiers == 0 || h.suppressedUntil.IsZero() {
		return
	}

	if time.Now().Before(h.suppressedUntil) {
		return
	}

	h.suppressedModifiers = 0
	h.suppressedUntil = time.Time{}
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
