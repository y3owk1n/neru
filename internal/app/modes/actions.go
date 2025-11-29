package modes

import (
	"context"

	"go.uber.org/zap"
)

// StartActionMode initiates standalone action mode with visual feedback.
func (h *Handler) StartActionMode() {
	h.cursorState.SkipNextRestore()
	// Reset any previous mode state
	h.ExitMode()

	// Enable event tap early to capture key presses immediately
	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	// Draw action highlight overlay
	h.drawActionHighlight()
	h.overlayManager.Show()

	h.SetModeAction()

	h.logger.Info("Action mode activated")
	h.logger.Info("Press action keys: l(left), r(right), m(middle), etc. Esc to exit")
}

// drawActionHighlight draws a highlight border around the active screen for action mode.
// This is used by both hints and grid modes when in action mode.
func (h *Handler) drawActionHighlight() {
	ctx := context.Background()

	showActionHighlightErr := h.actionService.ShowActionHighlight(ctx)
	if showActionHighlightErr != nil {
		h.logger.Error("Failed to draw action highlight", zap.Error(showActionHighlightErr))
	}
}
