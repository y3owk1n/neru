package modes

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
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

	uiConfig := h.config.StickyModifiers.UI
	xOffset := xCoordinate + uiConfig.IndicatorXOffset
	yOffset := yCoordinate + uiConfig.IndicatorYOffset

	h.stickyIndicatorService.UpdateIndicatorPosition(xOffset, yOffset, symbols)
}

func (h *Handler) drawStickyModifiersIndicatorAtCurrentCursor() {
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

	ctx, cancel := context.WithTimeout(context.Background(), stickyIndicatorTimeout)
	defer cancel()

	cursorX, cursorY, err := h.stickyIndicatorService.GetCursorPosition(ctx)
	if err != nil {
		return
	}

	h.drawStickyModifiersIndicator(cursorX, cursorY)
}
