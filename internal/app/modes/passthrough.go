package modes

import (
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
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

	// Resolve the focused app bundle ID once so both the blacklist and the
	// intercepted modifier key list use a consistent snapshot. The query is
	// only needed for hints mode — other modes have no per-app overrides.
	var bundleID string
	if enabled && mode == domain.ModeHints && h.config.Hints.HasAppHotkeyOverrides() {
		bundleID = h.focusedBundleID()
	}

	if h.setPassthroughCallback != nil {
		h.setPassthroughCallback(h.passthroughCallbackFor(mode, enabled))
	}

	if h.setModifierPassthrough != nil {
		blacklist := []string(nil)
		if enabled {
			blacklist = append(blacklist, h.config.General.PassthroughUnboundedKeysBlacklist...)

			// Hotkeys for the current mode must also be blacklisted
			// so the event tap consumes them instead of passing them through.
			hotkeys := h.config.HotkeysForModeAndApp(domain.ModeString(mode), bundleID)
			for key := range hotkeys {
				blacklist = append(blacklist, configpkg.NormalizeKeyForComparison(key))
			}
		}

		h.setModifierPassthrough(enabled, blacklist)
	}

	if h.setInterceptedModifierKeys == nil {
		return
	}

	keys := []string(nil)
	if enabled {
		keys = h.modeModifierKeys(mode, bundleID)
	}

	h.setInterceptedModifierKeys(keys)
}

func (h *Handler) passthroughCallbackFor(mode domain.Mode, enabled bool) func() {
	if !enabled {
		return nil
	}

	session := h.modeSession

	return func() {
		h.handlePassthrough(mode, session)
	}
}

const initialCapacity = 16

func (h *Handler) modeModifierKeys(mode domain.Mode, bundleID string) []string {
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

		keys = append(keys, normalized)
	}

	// Append hotkey keys for the current mode so the event tap
	// intercepts them instead of passing them through to macOS.
	hotkeys := h.config.HotkeysForModeAndApp(domain.ModeString(mode), bundleID)
	for key := range hotkeys {
		appendKey(key)
	}

	slices.Sort(keys)

	return keys
}

// handlePassthrough is called when a modifier shortcut was passed through to
// macOS while a mode was active. The callback carries the mode/session that
// were current when the event tap observed the passthrough so late callbacks
// cannot act on a different activation.
func (h *Handler) handlePassthrough(mode domain.Mode, session uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlePassthroughLocked(mode, session)
}

// handlePassthroughLocked is called when a modifier shortcut was passed through
// to macOS while a mode was active. The mode/session arguments identify the
// originating activation so stale callbacks can be ignored safely. Only hints
// mode needs a refresh because its labels point at AX elements that may have
// moved (e.g., Cmd+Tab switched the focused app). Grid, recursive-grid, and
// scroll modes use screen coordinates that remain valid regardless of what the
// OS does with the shortcut.
//
// Caller must hold h.mu.
func (h *Handler) handlePassthroughLocked(mode domain.Mode, session uint64) {
	if h.modeSession != session || h.appState.CurrentMode() != mode {
		return
	}

	h.cancelPendingModifierToggle()

	if h.config != nil && h.config.General.ShouldExitAfterPassthrough {
		h.logger.Debug("Exiting mode after passthrough",
			zap.String("mode", domain.ModeString(mode)),
			zap.Uint64("session", session))
		h.exitModeLocked()

		return
	}

	if mode != domain.ModeHints {
		return
	}

	h.logger.Debug("Scheduling hint refresh after modifier passthrough")

	// Cancel any existing refresh timer to debounce rapid passthroughs.
	if h.refreshHintsTimer != nil {
		h.refreshHintsTimer.Stop()
	}

	var timer *time.Timer

	timerSession := h.modeSession

	timer = time.AfterFunc(passthroughHintRefreshDelay, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		// Guard against stale timer: if the user exited hints mode while we
		// were waiting, or if hints was re-entered (new session), do not
		// re-activate.
		if h.modeSession != timerSession || h.appState.CurrentMode() != domain.ModeHints {
			return
		}

		// Clear our own timer reference only if we are still the active one.
		if h.refreshHintsTimer == timer {
			h.refreshHintsTimer = nil
		}

		h.logger.Debug("Refreshing hints after modifier passthrough",
			zap.Duration("delay", passthroughHintRefreshDelay))
		h.activateHintModeInternal(false, nil, nil)
	})
	h.refreshHintsTimer = timer
}
