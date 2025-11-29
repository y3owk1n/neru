package modes

import (
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// actionModeContext defines the interface for contexts that support action mode toggling.
type actionModeContext interface {
	PendingAction() *string
	InActionMode() bool
	SetInActionMode(inActionMode bool)
}

// toggleActionMode toggles between overlay and action mode for modes that support it.
func (h *Handler) toggleActionMode(
	ctx actionModeContext,
	reactivateFunc func(),
	overlayMode overlay.Mode,
	modeName string,
) {
	// Skip tab handling if pending action is set
	if ctx.PendingAction() != nil {
		h.logger.Debug("Tab key disabled when action is pending")

		return
	}

	if ctx.InActionMode() {
		ctx.SetInActionMode(false)

		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}
		// Re-activate mode while preserving action mode state
		reactivateFunc()
		h.logger.Info("Switched back to " + modeName + " overlay mode")
		h.overlaySwitch(overlayMode)
	} else {
		ctx.SetInActionMode(true)
		h.overlayManager.Clear()
		h.overlayManager.Hide()
		h.drawActionHighlight()
		h.overlayManager.Show()
		h.logger.Info("Switched to " + modeName + " action mode")
		h.overlaySwitch(overlay.ModeAction)
	}
}

// toggleActionModeForHints toggles between overlay and action mode for hints.
func (h *Handler) toggleActionModeForHints() {
	h.toggleActionMode(
		h.hints.Context,
		func() { h.activateHintModeInternal(true, nil) },
		overlay.ModeHints,
		modeNameHints,
	)
}

// toggleActionModeForGrid toggles between overlay and action mode for grid.
func (h *Handler) toggleActionModeForGrid() {
	h.toggleActionMode(
		h.grid.Context,
		func() { h.activateGridModeWithAction(nil) },
		overlay.ModeGrid,
		modeNameGrid,
	)
}
