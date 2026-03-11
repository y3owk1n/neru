package modes

import (
	"slices"
	"strings"
	"time"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// passthroughHintRefreshDelay is the delay before refreshing hints after a
// modifier shortcut is passed through to macOS. This gives the OS time to
// process the shortcut (e.g., Cmd+Tab app switch) before Neru re-collects
// AX elements.
const passthroughHintRefreshDelay = 300 * time.Millisecond

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

const initialCapacity = 16

func (h *Handler) modeModifierKeys(mode domain.Mode) []string {
	if h.config == nil || mode == domain.ModeIdle {
		return nil
	}

	keys := make([]string, 0, initialCapacity)
	seen := make(map[string]struct{}, initialCapacity)

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
	case domain.ModeIdle:
	case domain.ModeHints:
		appendKeys(h.config.Hints.ModeExitKeys)
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.Hints.BackspaceKey)
	case domain.ModeGrid:
		appendKeys(h.config.Grid.ModeExitKeys)
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.Grid.ResetKey)
		appendKey(h.config.Grid.BackspaceKey)
	case domain.ModeRecursiveGrid:
		appendKeys(h.config.RecursiveGrid.ModeExitKeys)
		appendActionModifierKeys(h.config.Action.KeyBindings, appendKey)
		appendKey(h.config.RecursiveGrid.ResetKey)
		appendKey(h.config.RecursiveGrid.BackspaceKey)
	case domain.ModeScroll:
		appendKeys(h.config.Scroll.ModeExitKeys)

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

// HandlePassthrough is the public entry point called when a modifier shortcut
// was passed through to macOS while a mode is active. It acquires h.mu and
// delegates to handlePassthroughLocked.
func (h *Handler) HandlePassthrough() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlePassthroughLocked()
}

// handlePassthroughLocked is called when a modifier shortcut was passed through
// to macOS while a mode is active. Only hints mode needs a refresh because its
// labels point at AX elements that may have moved (e.g., Cmd+Tab switched the
// focused app). Grid, recursive-grid, and scroll modes use screen coordinates
// that remain valid regardless of what the OS does with the shortcut.
//
// Caller must hold h.mu.
func (h *Handler) handlePassthroughLocked() {
	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}

	h.logger.Debug("Scheduling hint refresh after modifier passthrough")

	// Cancel any existing refresh timer to debounce rapid passthroughs.
	if h.refreshHintsTimer != nil {
		h.refreshHintsTimer.Stop()
	}

	var timer *time.Timer

	timer = time.AfterFunc(passthroughHintRefreshDelay, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		// Guard against stale timer: if the user exited hints mode while we
		// were waiting, do not re-activate.
		if h.appState.CurrentMode() != domain.ModeHints {
			return
		}

		// Clear our own timer reference only if we are still the active one.
		if h.refreshHintsTimer == timer {
			h.refreshHintsTimer = nil
		}

		h.logger.Debug("Refreshing hints after modifier passthrough",
			zap.Duration("delay", passthroughHintRefreshDelay))
		h.activateHintModeInternal(false, nil)
	})
	h.refreshHintsTimer = timer
}
