package modes

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
)

func (h *Handler) redrawMonitorSelectLocked() {
	if h.monitorSelect == nil {
		return
	}

	err := h.showMonitorSelectLocked()
	if err != nil {
		h.logger.Debug("Failed to redraw monitor_select overlay", zap.Error(err))
	}
}

// RefreshMonitorSelectForThemeChange redraws the monitor_select overlay using
// the latest theme-resolved colors when the mode is active.
func (h *Handler) RefreshMonitorSelectForThemeChange() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeMonitorSelect || h.monitorSelect == nil {
		return
	}

	h.redrawMonitorSelectLocked()
}
