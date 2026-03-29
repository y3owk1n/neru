package modes

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/ui/overlay"
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

	h.exitModeLocked()
}

// exitModeLocked exits the current mode. Caller must hold h.mu.
func (h *Handler) exitModeLocked() {
	if h.appState.CurrentMode() == domain.ModeIdle {
		return
	}

	h.logger.Info("Exiting current mode", zap.String("mode", h.CurrModeString()))

	h.performModeSpecificCleanup()
	h.performCommonCleanup()
	h.handleCursorRestoration()
}

// performModeSpecificCleanup handles mode-specific cleanup logic.
func (h *Handler) performModeSpecificCleanup() {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		h.cleanupDefaultMode()

		return
	}

	mode.Exit()
}

// clearAndHideOverlay clears and hides the overlay manager.
func (h *Handler) clearAndHideOverlay() {
	h.stopIndicatorPolling()

	h.overlayManager.Clear()
	h.overlayManager.Hide()
}

// cleanupHintsMode handles cleanup for hints mode.
func (h *Handler) cleanupHintsMode() {
	h.hints.Context.Reset()

	h.clearAndHideOverlay()
}

// cleanupDefaultMode handles cleanup for default/unknown modes.
func (h *Handler) cleanupDefaultMode() {
	// No domain-specific cleanup for other modes yet.
	// But still clear and hide action overlay.
	if overlay.Get() != nil {
		overlay.Get().Clear()
		overlay.Get().Hide()
	}
}

// cleanupGridMode handles cleanup for grid mode.
func (h *Handler) cleanupGridMode() {
	// Only reset the base context fields (pendingAction, repeat).
	// Do NOT call h.grid.Context.Reset() because it nils out
	// gridInstance (a **domainGrid.Grid pointer-to-pointer that is
	// wired once during component setup in component_factory.go).
	// Nilling it causes a nil-pointer dereference on re-activation
	// when SetGridInstanceValue dereferences the pointer.
	h.grid.Context.SetPendingAction(nil)
	h.grid.Context.SetRepeat(false)

	if h.grid.Manager != nil {
		h.grid.Manager.Reset()
	}

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
func (h *Handler) performCommonCleanup() {
	h.stopIndicatorPolling()
	h.overlayManager.Clear()

	// Stop any pending hints refresh timer to prevent re-activation after exit
	if h.refreshHintsTimer != nil {
		h.refreshHintsTimer.Stop()
		h.refreshHintsTimer = nil
	}

	h.hotkeyLastKey = ""
	h.hotkeyLastKeyTime = 0

	if h.disableEventTap != nil {
		h.disableEventTap()
	}

	accessibility.EnsureMouseUp()

	h.setAppModeLocked(domain.ModeIdle)
	h.suppressedModifiers = 0
	h.suppressedUntil = time.Time{}
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
func (h *Handler) handleCursorRestoration() {
	h.cursorState.Reset()

	// Always reset scroll context to ensure proper state cleanup when switching modes.
	h.scroll.Context.Reset()
}
