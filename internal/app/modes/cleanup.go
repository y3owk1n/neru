package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// ExitMode exits the current mode.
func (h *Handler) ExitMode() {
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
	h.overlayManager.Clear()
	h.overlayManager.Hide()
}

// cleanupHintsMode handles cleanup for hints mode.
func (h *Handler) cleanupHintsMode() {
	h.hints.Context.SetInActionMode(false)
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
	h.grid.Context.SetInActionMode(false)

	if h.grid.Manager != nil {
		h.grid.Manager.Reset()
	}

	h.clearAndHideOverlay()
}

// performCommonCleanup handles common cleanup logic for all modes.
func (h *Handler) performCommonCleanup() {
	h.overlayManager.Clear()

	if h.disableEventTap != nil {
		h.disableEventTap()
	}

	h.appState.SetMode(domain.ModeIdle)
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

// handleCursorRestoration handles cursor position restoration on exit.
func (h *Handler) handleCursorRestoration() {
	shouldRestore := h.shouldRestoreCursorOnExit()
	if shouldRestore {
		currentBounds := bridge.ActiveScreenBounds()
		target := coordinates.ComputeRestoredPosition(
			h.cursorState.InitialPosition(),
			h.cursorState.InitialScreenBounds(),
			currentBounds,
		)
		ctx := context.Background()

		restoreCursorErr := h.actionService.MoveCursorToPoint(ctx, target)
		if restoreCursorErr != nil {
			h.logger.Error("Failed to restore cursor position", zap.Error(restoreCursorErr))
		}
	}

	h.cursorState.Reset()
	// Always reset scroll context regardless of whether we performed cursor restoration.
	// This ensures proper state cleanup when switching between modes.
	h.scroll.Context.SetIsActive(false)
	h.scroll.Context.SetLastKey("")
}
