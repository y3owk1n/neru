package modes

import (
	"context"

	"go.uber.org/zap"
)

// drawActionHighlight draws a highlight border around the active screen for action mode.
// This is used by both hints and grid modes when in action mode.
func (h *Handler) drawActionHighlight() {
	ctx := context.Background()

	showActionHighlightErr := h.actionService.ShowActionHighlight(ctx)
	if showActionHighlightErr != nil {
		h.logger.Error("Failed to draw action highlight", zap.Error(showActionHighlightErr))
	}
}
