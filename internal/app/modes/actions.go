package modes

import (
	"context"

	"go.uber.org/zap"
)

// drawActionHighlight draws a highlight border around the active screen for action mode.
// This is used by both hints and grid modes when in action mode.
func (h *Handler) drawActionHighlight() {
	ctx := context.Background()
	if err := h.ActionService.ShowActionHighlight(ctx); err != nil {
		h.Logger.Error("Failed to draw action highlight", zap.Error(err))
	}
}
