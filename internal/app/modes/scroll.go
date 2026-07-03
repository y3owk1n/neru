package modes

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// StartInteractiveScroll activates the interactive scroll mode,
// showing the scroll overlay and enabling key handling for scrolling.
func (h *Handler) StartInteractiveScroll() {
	h.cursorState.SkipNextRestore()

	h.scroll.Context.Reset()

	if h.appState.CurrentMode() != domain.ModeIdle {
		// Mode-to-mode transition: clean up the current mode but keep the
		// event tap enabled. Skipping disableEventTap avoids a brief dead
		// window where CGEventTap silently drops key events between the
		// disable and subsequent re-enable (the root cause of missed
		// scrolling keys when activating from grid mode).
		h.performModeSpecificCleanup()
		h.stopHeldRepeatLocked()
		h.overlayManager.Clear()
		h.overlayManager.ClearCache()

		if h.refreshHintsTimer != nil {
			h.refreshHintsTimer.Stop()
			h.refreshHintsTimer = nil
		}

		h.hotkeyLastKey = ""
		h.hotkeyLastKeyTime = 0
		h.clearStickyModifiers()
		accessibility.EnsureMouseUp()

		h.suppressedModifiers = 0
		h.suppressedUntil = time.Time{}
		h.cursorState.Reset()

		if h.appState.HotkeyRefreshPending() {
			h.appState.SetHotkeyRefreshPending(false)

			if h.refreshHotkeys != nil {
				go h.refreshHotkeys()
			}
		}

		h.logger.Debug("Transitioned to scroll mode",
			zap.String("from", h.CurrModeString()))
	}

	h.scroll.Context.SetIsActive(true)

	h.overlayManager.ResizeToActiveScreen()

	h.setModeLocked(domain.ModeScroll, overlay.ModeScroll)

	h.logger.Info("Interactive scroll activated")
}

// handleGenericScrollKey intentionally does nothing.
// Scroll key behavior is fully driven by hotkeys.
func (h *Handler) handleGenericScrollKey(_ string) {}
