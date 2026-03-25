package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// StartInteractiveScroll activates the interactive scroll mode,
// showing the scroll overlay and enabling key handling for scrolling.
func (h *Handler) StartInteractiveScroll() {
	h.cursorState.SkipNextRestore()

	// Defensively reset scroll context before exiting the current mode.
	// exitModeLocked returns early when already idle without running cleanup.
	h.scroll.Context.Reset()

	h.exitModeLocked()

	h.scroll.Context.SetIsActive(true)

	h.overlayManager.ResizeToActiveScreen()

	h.setModeLocked(domain.ModeScroll, overlay.ModeScroll)

	h.logger.Info("Interactive scroll activated")
	h.logger.Info("Use configured keys for navigation")
}

// handleGenericScrollKey intentionally does nothing.
// Scroll key behavior is fully driven by hotkeys.
func (h *Handler) handleGenericScrollKey(_ string) {}
