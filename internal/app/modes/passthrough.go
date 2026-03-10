package modes

import (
	"slices"
	"strings"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func (h *Handler) syncModifierPassthrough(mode domain.Mode) {
	enabled := h.config != nil &&
		mode != domain.ModeIdle &&
		h.config.General.PassthroughUnboundedKeys

	if h.setModifierPassthrough != nil {
		blacklist := []string(nil)
		if enabled {
			blacklist = append(blacklist, h.config.General.PassthroughUnboundedKeysBlacklist...)
		}

		h.setModifierPassthrough(enabled, blacklist)
	}

	if h.setInterceptedModifierKeys == nil {
		return
	}

	keys := []string(nil)
	if enabled {
		keys = h.modeModifierKeys(mode)
	}

	h.setInterceptedModifierKeys(keys)
}

func (h *Handler) modeModifierKeys(mode domain.Mode) []string {
	if h.config == nil || mode == domain.ModeIdle {
		return nil
	}

	keys := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)

	appendKey := func(key string) {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || !configpkg.HasPassthroughModifier(trimmed) {
			return
		}

		normalized := configpkg.NormalizeKeyForComparison(trimmed)
		if _, exists := seen[normalized]; exists {
			return
		}

		seen[normalized] = struct{}{}
		keys = append(keys, trimmed)
	}

	appendKeys := func(values []string) {
		for _, value := range values {
			appendKey(value)
		}
	}

	appendKeys(h.config.General.ModeExitKeys)

	switch mode {
	case domain.ModeHints:
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.Hints.BackspaceKey)
	case domain.ModeGrid:
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.Grid.ResetKey)
		appendKey(h.config.Grid.BackspaceKey)
	case domain.ModeRecursiveGrid:
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.RecursiveGrid.ResetKey)
		appendKey(h.config.RecursiveGrid.BackspaceKey)
	case domain.ModeScroll:
		for _, bindings := range h.config.Scroll.KeyBindings {
			appendKeys(bindings)
		}
	}

	slices.Sort(keys)

	return keys
}

func appendActionModifierKeys(bindings configpkg.ActionKeyBindingsCfg, appendKey func(string)) {
	appendKey(bindings.LeftClick)
	appendKey(bindings.RightClick)
	appendKey(bindings.MiddleClick)
	appendKey(bindings.MouseDown)
	appendKey(bindings.MouseUp)
	appendKey(bindings.MoveMouseUp)
	appendKey(bindings.MoveMouseDown)
	appendKey(bindings.MoveMouseLeft)
	appendKey(bindings.MoveMouseRight)
}
