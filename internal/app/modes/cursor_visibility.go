package modes

import (
	"time"

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

// cursorRehideInterval is the minimum interval between automatic cursor re-hide
// calls in the indicator polling loop. This prevents excessive main-queue
// dispatches while ensuring the cursor is re-hidden within ~500ms after
// macOS reveals it (e.g. right-click context menus, Mission Control).
const cursorRehideInterval = 500 * time.Millisecond

// rehideSystemCursor re-hides the system cursor if it may have been
// revealed by a system event (right-click, Mission Control, Exposé, etc.).
// Uses TryLock internally to avoid deadlocks with the polling goroutine.
// Respects the rate limit defined by cursorRehideInterval.
func (h *Handler) rehideSystemCursor() {
	if !h.mu.TryLock() {
		return
	}
	defer h.mu.Unlock()

	if !h.systemCursorHidden {
		return
	}

	if time.Since(h.lastCursorRehideTime) < cursorRehideInterval {
		return
	}

	h.lastCursorRehideTime = time.Now()
	h.RehideSystemCursor()
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
