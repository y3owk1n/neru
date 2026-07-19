package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// HideSystemCursor hides the system cursor and shows the virtual pointer when enabled.
func (h *Handler) HideSystemCursor() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.hideSystemCursorLocked()
}

// hideSystemCursorLocked hides the system cursor. Caller must hold h.mu.
func (h *Handler) hideSystemCursorLocked() {
	if h.systemCursorHidden {
		return
	}

	h.hideSystemCursorNative()
	h.systemCursorHidden = true
	h.ensureCursorOverlayPollingLocked()
}

// ShowSystemCursor shows the system cursor and hides the cursor-following virtual pointer.
func (h *Handler) ShowSystemCursor() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.showSystemCursorLocked()
}

// showSystemCursorLocked shows the system cursor. Caller must hold h.mu.
func (h *Handler) showSystemCursorLocked() {
	if !h.systemCursorHidden {
		return
	}

	h.showSystemCursorNative()
	h.systemCursorHidden = false
	h.hideCursorFollowingVirtualPointerLocked()
	h.stopCursorOverlayPollingIfIdleLocked()
}

func (h *Handler) shouldShowCursorFollowingVirtualPointerLocked() bool {
	return h.systemCursorHidden && h.config != nil
}

func (h *Handler) ensureCursorOverlayPollingLocked() {
	if !h.shouldShowCursorFollowingVirtualPointerLocked() {
		return
	}

	h.startIndicatorPolling(h.appState.CurrentMode())
}

func (h *Handler) stopCursorOverlayPollingIfIdleLocked() {
	if h.shouldPollCursorOverlaysLocked(h.appState.CurrentMode()) {
		return
	}

	h.stopIndicatorPolling()
}

func (h *Handler) shouldPollCursorOverlaysLocked(mode domain.Mode) bool {
	if h.config == nil {
		return false
	}

	if h.stickyModifiersEnabled() {
		return true
	}

	if h.modeIndicatorEnabled(mode) {
		return true
	}

	return h.shouldShowCursorFollowingVirtualPointerLocked()
}

func (h *Handler) hideCursorFollowingVirtualPointerLocked() {
	if h.overlayManager == nil {
		return
	}

	if vp := h.overlayManager.VirtualPointerOverlay(); vp != nil {
		vp.Clear()
		vp.Hide()
	}
}
