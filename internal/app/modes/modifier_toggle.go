package modes

import (
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const modifierTogglePrefix = "__modifier_"

// modifierToggleDebounce is the delay between a modifier key-up and the actual
// sticky toggle. This allows remapped keys (e.g., Karabiner Option+h → Left)
// to arrive and cancel the pending toggle before it fires. 50ms is long enough
// for the remapped key event to arrive but short enough to feel instantaneous.
const modifierToggleDebounce = 50 * time.Millisecond

// defaultModifierToggleCooldown is the fallback cooldown when the config value
// is zero (disabled). Kept as a named constant for documentation; in practice
// the cooldown is only applied when the user explicitly sets tap_cooldown > 0.
const defaultModifierToggleCooldown = 0

var modifierToggleMap = map[string]action.Modifiers{
	"cmd":   action.ModCmd,
	"shift": action.ModShift,
	"alt":   action.ModAlt,
	"ctrl":  action.ModCtrl,
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
//
// The logic is simple and deterministic:
//   - On modifier keydown: record which modifier is pending.
//   - On modifier keyup: if the same modifier is still pending (no regular key
//     was pressed in between), toggle it as sticky. Otherwise ignore.
//   - Any regular key press cancels the pending modifier (see key_dispatch.go).
//
// This means Shift↓ → Shift↑ toggles sticky, but Shift↓ → L → Shift↑ does not.
func (h *Handler) handleModifierToggle(key string) bool {
	if !h.stickyModifiersEnabled() {
		return false
	}

	mod, isDown, ok := parseModifierEvent(key)
	if !ok {
		return false
	}

	// Modifier detection is disarmed on mode entry and re-armed once we see
	// a _up event (meaning all activation-related modifiers have been released
	// and the user is starting fresh). Until armed, consume events silently.
	if !h.modifierDetectionArmed {
		if !isDown {
			// A key-up means the user released a modifier — arm detection
			// so the next intentional down/up pair will be processed.
			h.modifierDetectionArmed = true
			h.logger.Debug("Modifier detection armed (first key-up after mode entry)",
				zap.String("key", key))
		} else {
			h.logger.Debug("Modifier event ignored (detection not armed)",
				zap.String("key", key))
		}

		return true
	}

	// Normalize to lowercase for consistent matching (parseModifierEvent also
	// lowercases), so mixed-case event sources can never cause a silent mismatch.
	normalizedKey := strings.ToLower(key)

	if isDown {
		if h.pendingModifierKeys == nil {
			h.pendingModifierKeys = make(map[string]time.Time)
		}

		h.pendingModifierKeys[normalizedKey] = time.Now()
		h.logger.Debug("Modifier key down", zap.String("key", normalizedKey))

		return true
	}

	// Key up — schedule a debounced toggle only if the matching down is still pending.
	// The debounce allows remapped keys (e.g., Karabiner) to cancel the pending toggle.
	expectedDown := strings.TrimSuffix(normalizedKey, "_up") + "_down"
	downTime, pending := h.pendingModifierKeys[expectedDown]

	if !pending {
		h.logger.Debug("Modifier key up ignored (no matching pending down)",
			zap.String("key", key),
			zap.Any("pending", h.pendingModifierKeys))

		return true
	}

	// If a tap-max-duration is configured, reject holds that exceeded it
	// before even scheduling the debounce.
	if maxDur := h.config.StickyModifiers.TapMaxDuration; maxDur > 0 {
		elapsed := time.Since(downTime)
		if elapsed > time.Duration(maxDur)*time.Millisecond {
			delete(h.pendingModifierKeys, expectedDown)
			h.logger.Debug("Modifier tap rejected (held too long)",
				zap.String("modifier", mod.String()),
				zap.Duration("held", elapsed),
				zap.Int("maxMs", maxDur))

			return true
		}
	}

	// Schedule a debounced toggle. The pending down entry stays in the map so
	// that a regular key press arriving during the debounce window can cancel
	// it via cancelPendingModifierToggle.
	h.scheduleModifierToggle(expectedDown, mod)

	return true
}

// scheduleModifierToggle starts a debounce timer that will toggle the given
// modifier after modifierToggleDebounce unless canceled by a regular key press.
func (h *Handler) scheduleModifierToggle(expectedDown string, mod action.Modifiers) {
	if h.pendingModifierTimers == nil {
		h.pendingModifierTimers = make(map[string]*time.Timer)
	}

	// Cancel any existing timer for this modifier (shouldn't happen, but be safe).
	if existingTimer, exists := h.pendingModifierTimers[expectedDown]; exists {
		existingTimer.Stop()
	}

	h.logger.Debug("Scheduling modifier toggle debounce",
		zap.String("modifier", mod.String()),
		zap.Duration("delay", modifierToggleDebounce))

	timer := time.AfterFunc(modifierToggleDebounce, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		// If the pending down was already canceled (by a regular key press),
		// the entry will be gone from the map — do nothing.
		downTime, stillPending := h.pendingModifierKeys[expectedDown]
		if !stillPending {
			h.logger.Debug("Modifier toggle debounce canceled (regular key intervened)",
				zap.String("modifier", mod.String()))

			delete(h.pendingModifierTimers, expectedDown)

			return
		}

		// Suppress toggle if a regular key was pressed shortly before the
		// modifier went down. We measure relative to the modifier-down time
		// (not "now") so that long modifier holds don't defeat the cooldown.
		// This catches both rapid Karabiner usage and ghost modifier events
		// where Karabiner's internal timeout (~250ms) elapses.
		// Only applied when tap_cooldown > 0 in config (default 0 = disabled).
		cooldownMs := defaultModifierToggleCooldown
		if h.config != nil {
			cooldownMs = h.config.StickyModifiers.TapCooldown
		}

		if cooldownMs > 0 && !h.lastRegularKeyTime.IsZero() &&
			downTime.Sub(h.lastRegularKeyTime) < time.Duration(cooldownMs)*time.Millisecond {
			h.logger.Debug("Modifier toggle suppressed (key activity before modifier press)",
				zap.String("modifier", mod.String()),
				zap.Duration("key_to_mod_gap", downTime.Sub(h.lastRegularKeyTime)),
				zap.Int("cooldownMs", cooldownMs))

			delete(h.pendingModifierKeys, expectedDown)
			delete(h.pendingModifierTimers, expectedDown)

			return
		}

		delete(h.pendingModifierKeys, expectedDown)
		delete(h.pendingModifierTimers, expectedDown)

		newModifiers := h.modifierState.Toggle(mod)
		h.logger.Debug("Sticky modifier toggled (after debounce)",
			zap.String("modifier", mod.String()),
			zap.String("state", newModifiers.String()))
	})

	h.pendingModifierTimers[expectedDown] = timer
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
