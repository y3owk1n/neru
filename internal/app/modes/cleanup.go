package modes

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	// postActionSettleDelay is the time to wait after a click action completes
	// before moving the cursor for restoration/centering. This gives the target
	// application time to finish processing the mouseUp event. Without this
	// delay, cursor restoration can race with click processing in slow apps
	// (Electron, web views) causing missed clicks.
	postActionSettleDelay = 75 * time.Millisecond
)

// ExitMode exits the current mode. Safe to call from any goroutine.
func (h *Handler) ExitMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.exitMode()
}

// exitMode exits the current mode. Caller must hold h.mu.
func (h *handlerState) exitMode() {
	if h.appState.CurrentMode() == domain.ModeIdle {
		return
	}

	h.logger.Debug("Exiting current mode", zap.String("mode", h.CurrModeString()))

	h.performModeSpecificCleanup()
	h.performCommonCleanup()
	h.handleCursorRestoration()
}

// performModeSpecificCleanup handles mode-specific cleanup logic.
func (h *handlerState) performModeSpecificCleanup() {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		h.cleanupDefaultMode()

		return
	}

	mode.Exit()
}

// clearAndHideOverlay clears and hides the overlay manager.
func (h *handlerState) clearAndHideOverlay() {
	h.stopIndicatorPolling()

	h.overlayManager.ClearCache()
	h.overlayManager.Hide()
}

// cleanupHintsMode handles cleanup for hints mode.
func (h *handlerState) cleanupHintsMode() {
	h.stopHintSearchTextInput(false)

	resetErr := h.hints.Context.Reset()
	if resetErr != nil {
		h.logger.Error("Failed to reset hints context", zap.Error(resetErr))
	}

	h.cycleHintIndex = -1

	h.clearAndHideOverlay()
}

// cleanupDefaultMode handles cleanup for default/unknown modes.
func (h *handlerState) cleanupDefaultMode() {
	// No domain-specific cleanup for other modes yet.
	// But still clear and hide action overlay.
	if overlay.Get() != nil {
		overlay.Get().Clear()
		overlay.Get().Hide()
	}
}

// cleanupGridMode handles cleanup for grid mode.
func (h *handlerState) cleanupGridMode() {
	// Only reset the base context fields (pendingAction, repeat).
	// Do NOT call h.grid.Context.Reset() because it nils out
	// gridInstance (a **domainGrid.Grid pointer-to-pointer that is
	// wired once during component setup in component_factory.go).
	// Nilling it causes a nil-pointer dereference on re-activation
	// when SetGridInstanceValue dereferences the pointer.
	h.grid.Context.SetPendingAction(nil)
	h.grid.Context.SetOnExit(nil)
	h.grid.Context.SetRepeat(false)

	if h.grid.Manager != nil {
		h.grid.Manager.ResetSilent()
	}

	// Reset renderer match state so the native hideUnmatched flag and
	// stale cell-match data do not persist across activations. Normally
	// the update callback (fired by Reset()) would do this, but
	// ResetSilent() deliberately skips it.
	h.renderer.SetHideUnmatched(false)
	h.renderer.UpdateGridMatches("")

	// Explicitly hide the virtual pointer before clearing the overlay.
	// NeruClearOverlay also resets cursorIndicatorVisible, but we do this
	// explicitly so the pointer cleanup does not silently depend on the
	// overlay clear implementation.
	if h.grid.Overlay != nil {
		h.grid.Overlay.HideVirtualPointer()
	}

	h.clearAndHideOverlay()
}

// performCommonCleanup handles common cleanup logic for all modes.
func (h *handlerState) performCommonCleanup() {
	h.stopIndicatorPolling()
	h.stopHeldRepeat()
	h.overlayManager.Clear()
	h.overlayManager.ClearCache()

	// Stop any pending hints refresh timer to prevent re-activation after exit
	if h.refreshHintsTimer != nil {
		h.refreshHintsTimer.Stop()
		h.refreshHintsTimer = nil
	}

	h.hotkeyLastKey = ""
	h.hotkeyLastKeyTime = 0

	// Release synthetic sticky modifiers before disabling the Wayland event tap.
	// The evdev-backed tap restores overlay keyboard focus during shutdown; if
	// we post modifier key-up events after that handoff, the overlay can receive
	// the release instead of the target app and leave the app "stuck" with the
	// modifier still logically held.
	h.clearStickyModifiers()

	if h.hasEventTap() {
		h.disableEventTap()
	}

	h.releaseHeldButtons()

	h.setAppMode(domain.ModeIdle)

	// Do NOT reset suppressedModifiers here — SuppressModifiersForHotkey was
	// called synchronously by the hotkey dispatch path (dispatchModeAwareHeldHotkey
	// or dispatchModeAwareHotkeyAsync) before the mode switch, and the modifier
	// UP events arrive after the user releases the chord. Clearing
	// suppressedModifiers here causes the modifier DOWN events (for the next
	// chord press) to be handled as normal modifier taps instead of suppressed,
	// which schedules a debounce timer that fires and toggles the sticky modifier
	// when it shouldn't.
	//
	// expireSuppressedModifiersIfNeeded handles cleanup after the 2-second
	// activationModifierSuppressionWindow expires.
	// h.suppressedModifiers = 0
	// h.suppressedUntil = time.Time{}
	h.logger.Debug("Mode transition complete",
		zap.String("to", "idle"))
	h.overlayManager.SwitchTo(overlay.ModeIdle)

	// If a hotkey refresh was deferred while in an active mode, perform it now
	if h.appState.HotkeyRefreshPending() {
		h.appState.SetHotkeyRefreshPending(false)

		if h.refreshHotkeys != nil {
			go h.refreshHotkeys()
		}
	}
}

// handleCursorRestoration finalizes transient cursor and scroll state on mode exit.
func (h *handlerState) handleCursorRestoration() {
	h.cursorState.Reset()

	// Always reset scroll context to ensure proper state cleanup when switching modes.
	h.scroll.Context.Reset()
}

// releaseHeldButtons releases any mouse button left down by an interrupted
// drag. It routes through the action service rather than reaching into the
// accessibility infra package directly, and swallows the error because every
// caller is already on a cleanup path where there is nothing left to abort.
func (h *handlerState) releaseHeldButtons() {
	if h.actionService == nil {
		return
	}

	err := h.actionService.ReleaseHeldButtons(h.ctx)
	if err != nil {
		h.logger.Debug("Failed to release held mouse buttons", zap.Error(err))
	}
}
