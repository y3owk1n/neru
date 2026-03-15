package modes

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

const modifierTogglePrefix = "__modifier_"

const (
	stickyToggleDelay      = 150 * time.Millisecond
	stickyIndicatorTimeout = 100 * time.Millisecond
)

var modifierToggleMap = map[string]action.Modifiers{
	"cmd":   action.ModCmd,
	"shift": action.ModShift,
	"alt":   action.ModAlt,
	"ctrl":  action.ModCtrl,
}

var modifierSymbols = map[action.Modifiers]string{
	action.ModCmd:   "⌘",
	action.ModShift: "⇧",
	action.ModAlt:   "⌥",
	action.ModCtrl:  "⌃",
}

func modifierSymbolsString(mods action.Modifiers) string {
	if mods == 0 {
		return ""
	}

	var symbols string

	if mods.Has(action.ModCmd) {
		symbols += modifierSymbols[action.ModCmd]
	}

	if mods.Has(action.ModShift) {
		symbols += modifierSymbols[action.ModShift]
	}

	if mods.Has(action.ModAlt) {
		symbols += modifierSymbols[action.ModAlt]
	}

	if mods.Has(action.ModCtrl) {
		symbols += modifierSymbols[action.ModCtrl]
	}

	return symbols
}

func parseModifierToggleKey(key string) (action.Modifiers, bool) {
	if !strings.HasPrefix(key, modifierTogglePrefix) {
		return 0, false
	}

	suffix := strings.ToLower(strings.TrimPrefix(key, modifierTogglePrefix))

	mod, ok := modifierToggleMap[suffix]

	return mod, ok
}

func (h *Handler) handleModifierToggle(key string) bool {
	if !h.stickyModifiersEnabled() {
		return false
	}

	mod, isModifier := parseModifierToggleKey(key)
	if !isModifier {
		return false
	}

	pendingKey := key
	pendingMod := mod

	if h.pendingModifierToggle != nil {
		h.pendingModifierToggle.Stop()
		h.pendingModifierToggle = nil
	}

	h.pendingModifierToggle = time.AfterFunc(stickyToggleDelay, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		if h.pendingModifierKey == pendingKey {
			newModifiers := h.modifierState.Toggle(pendingMod)
			h.logger.Debug("Sticky modifier toggled",
				zap.String("modifier", pendingMod.String()),
				zap.String("state", newModifiers.String()))
			h.pendingModifierToggle = nil
			h.pendingModifierKey = ""
		}
	})

	h.pendingModifierKey = pendingKey

	h.logger.Debug("Modifier tap started", zap.String("key", key))

	return true
}

func (h *Handler) cancelPendingModifierToggle() {
	if h.pendingModifierToggle != nil {
		h.pendingModifierToggle.Stop()
		h.pendingModifierToggle = nil
		h.pendingModifierKey = ""
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
	if h.overlayManager == nil {
		return
	}

	if h.config == nil {
		return
	}

	if !h.config.StickyModifiers.Enabled {
		return
	}

	mods := h.stickyModifiers()
	if mods == 0 {
		return
	}

	symbols := modifierSymbolsString(mods)
	if symbols == "" {
		return
	}

	uiConfig := h.config.StickyModifiers.UI
	xOffset := xCoordinate + uiConfig.IndicatorXOffset
	yOffset := yCoordinate + uiConfig.IndicatorYOffset

	h.overlayManager.DrawStickyModifiersIndicator(xOffset, yOffset, symbols)
}

func (h *Handler) drawStickyModifiersIndicatorAtCurrentCursor() {
	if h.overlayManager == nil || h.system == nil {
		return
	}

	if h.config == nil || !h.config.StickyModifiers.Enabled {
		return
	}

	mods := h.stickyModifiers()
	if mods == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), stickyIndicatorTimeout)
	defer cancel()

	point, err := h.system.CursorPosition(ctx)
	if err != nil {
		return
	}

	h.drawStickyModifiersIndicator(point.X, point.Y)
}
