package modes

import (
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// passthroughHintRefreshDelay is the delay before refreshing hints after a
// modifier shortcut is passed through to macOS. This gives the OS time to
// process the shortcut (e.g., Cmd+Tab app switch) before Neru re-collects
// AX elements.
const passthroughHintRefreshDelay = 300 * time.Millisecond

// syncModifierPassthrough tells the event tap which keys the active mode
// answers to, so it consumes those and passes the rest through.
//
// Both lists it builds are the keys of the keymap in force, read rather than
// merged: this runs on the same triggers that settle one — a mode change, a
// configuration replacement, a hints refresh after the focused app changed — so
// there is nothing here to invalidate. Every caller passes the mode that is
// active, which is what makes the settled keymap the right one to read.
//
// Caller must hold h.mu.
func (h *handlerState) syncModifierPassthrough(mode domain.Mode) {
	if !h.hasEventTap() {
		return
	}

	enabled := h.config != nil &&
		mode != domain.ModeIdle &&
		h.config.General.PassthroughUnboundedKeys

	h.setPassthroughCallback(h.passthroughCallbackFor(mode, enabled))

	// The keymap is only consulted when passthrough is on: with it off the tap
	// consumes everything anyway, and a mode that is not open binds nothing.
	keymap := configpkg.Keymap{}
	blacklist := []string(nil)

	if enabled {
		keymap = h.settledKeymap()

		blacklist = append(blacklist, h.config.General.PassthroughUnboundedKeysBlacklist...)

		// The keys the mode binds must also be blacklisted so the event tap
		// consumes them instead of passing them through.
		for _, key := range keymap.Keys() {
			blacklist = append(blacklist, configpkg.CanonicalHotkeyForPlatform(key))
		}
	}

	h.setModifierPassthrough(enabled, blacklist)

	h.setInterceptedModifierKeys(modeModifierKeys(keymap))
}

func (h *handlerState) passthroughCallbackFor(mode domain.Mode, enabled bool) func() {
	if !enabled {
		return nil
	}

	session := h.modeSession

	return func() {
		h.outer.handlePassthrough(mode, session)
	}
}

const initialCapacity = 16

// modeModifierKeys is the modifier chords the keymap binds, in the form the
// platform canonicalizes them to: the event tap intercepts these instead of
// passing them through to the focused application.
func modeModifierKeys(keymap configpkg.Keymap) []string {
	if keymap.Len() == 0 {
		return nil
	}

	keys := make([]string, 0, initialCapacity)
	seen := make(map[string]struct{}, initialCapacity)

	for _, key := range keymap.Keys() {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || !configpkg.HasPassthroughModifier(trimmed) {
			continue
		}

		normalized := configpkg.CanonicalHotkeyForPlatform(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}

		keys = append(keys, normalized)
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

	h.passthroughTick(mode, session)
}

// passthroughTick is called when a modifier shortcut was passed through
// to macOS while a mode was active. The mode/session arguments identify the
// originating activation so stale callbacks can be ignored safely. Only hints
// mode needs a refresh because its labels point at AX elements that may have
// moved (e.g., Cmd+Tab switched the focused app). Grid, recursive-grid, and
// scroll modes use screen coordinates that remain valid regardless of what the
// OS does with the shortcut.
//
// Caller must hold h.mu.
func (h *handlerState) passthroughTick(mode domain.Mode, session uint64) {
	if h.modeSession != session || h.appState.CurrentMode() != mode {
		return
	}

	h.cancelPendingModifierToggle()

	if h.config != nil && h.config.General.ShouldExitAfterPassthrough {
		h.logger.Debug("Exiting mode after passthrough",
			zap.String("mode", domain.ModeString(mode)),
			zap.Uint64("session", session))
		h.exitMode()

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
		h.outer.mu.Lock()
		defer h.outer.mu.Unlock()

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
		filterRoles := h.hints.Context.FilterRoles()
		filterTextContains := h.hints.Context.FilterTextContains()
		startWithSearch := h.hints.Context.StartWithSearch()
		strategyOverride := h.hints.Context.StrategyOverride()
		labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
		splitWord := h.hints.Context.SplitWord()
		h.activateHintModeInternal(modecmd.Activation{
			Mode:               domain.ModeHints,
			FilterRoles:        filterRoles,
			FilterTextContains: filterTextContains,
			Search:             &startWithSearch,
			Strategy:           &strategyOverride,
			LabelDirection:     &labelDirectionOverride,
			SplitWord:          &splitWord,
			// OnExit is left nil to preserve the stored steps across refresh.
		})
	})
	h.refreshHintsTimer = timer
}
