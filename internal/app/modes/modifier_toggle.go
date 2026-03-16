package modes

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const modifierTogglePrefix = "__modifier_"

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
			h.pendingModifierKeys = make(map[string]bool)
		}

		h.pendingModifierKeys[normalizedKey] = true
		h.logger.Debug("Modifier key down", zap.String("key", normalizedKey))

		return true
	}

	// Key up — toggle only if the matching down is still pending.
	expectedDown := strings.TrimSuffix(normalizedKey, "_up") + "_down"
	if !h.pendingModifierKeys[expectedDown] {
		h.logger.Debug("Modifier key up ignored (no matching pending down)",
			zap.String("key", key),
			zap.Any("pending", h.pendingModifierKeys))

		return true
	}

	delete(h.pendingModifierKeys, expectedDown)

	newModifiers := h.modifierState.Toggle(mod)
	h.logger.Debug("Sticky modifier toggled",
		zap.String("modifier", mod.String()),
		zap.String("state", newModifiers.String()))

	return true
}

func (h *Handler) cancelPendingModifierToggle() {
	if len(h.pendingModifierKeys) > 0 {
		h.pendingModifierKeys = nil
		h.logger.Debug("Modifier tap canceled")
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
