package modes

import (
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// MonitorSelectMode implements the Mode interface for interactive monitor picking.
type MonitorSelectMode struct {
	*GenericMode
}

// NewMonitorSelectMode creates a new monitor_select mode implementation.
func NewMonitorSelectMode(handler *Handler) *MonitorSelectMode {
	return &MonitorSelectMode{
		GenericMode: NewGenericMode(
			handler,
			domain.ModeMonitorSelect,
			"MonitorSelectMode",
			ModeBehavior{
				ActivateFunc: func(handler *Handler, opts ModeActivationOptions) {
					handler.activateMonitorSelectMode(opts)
				},
				HandleKeyFunc: func(handler *Handler, key string) {
					handler.handleMonitorSelectKey(key)
				},
				ExitFunc: func(handler *Handler) {
					handler.cleanupMonitorSelectMode()
				},
			},
		),
	}
}

func (h *Handler) activateMonitorSelectMode(_ ModeActivationOptions) {
	err := h.validateModeActivation(
		"",
		domain.ModeNameMonitorSelect,
		h.config.MonitorSelect.Enabled,
	)
	if err != nil {
		h.logger.Debug("monitor_select activation rejected", zap.Error(err))

		return
	}

	h.prepareForModeActivation()

	monitors, currentBounds, err := h.discoverMonitorsForSelection()
	if err != nil {
		if derrors.IsNotSupported(err) {
			h.reportMonitorSelectNotSupported()
		} else {
			h.logger.Error("Failed to discover monitors for monitor_select", zap.Error(err))
		}

		return
	}

	session := newMonitorSelectSession(monitors, currentBounds, h.config.MonitorSelect)
	if session == nil {
		h.logger.Debug("Skipping monitor_select activation; no selectable monitors")

		return
	}

	h.exitModeLocked()
	h.monitorSelect = session

	err = h.showMonitorSelectLocked()
	if err != nil {
		h.monitorSelect = nil

		if derrors.IsNotSupported(err) {
			h.reportMonitorSelectNotSupported()
		} else {
			h.logger.Error("Failed to draw monitor_select overlay", zap.Error(err))
		}

		return
	}

	h.setModeLocked(domain.ModeMonitorSelect, overlay.ModeMonitorSelect)
	h.startIndicatorPolling(domain.ModeMonitorSelect)
	h.logger.Info("Monitor select mode activated", zap.Int("targets", len(session.targets)))
}

func (h *Handler) handleMonitorSelectKey(key string) {
	if h.monitorSelect == nil {
		return
	}

	if target := h.monitorSelect.HandleCharacter(key); target != nil {
		h.confirmMonitorSelectLocked(target)

		return
	}

	h.redrawMonitorSelectLocked()
}

func (h *Handler) confirmMonitorSelectLocked(target *monitorSelectTarget) {
	if target == nil {
		return
	}

	bounds := target.Bounds
	center := image.Point{
		X: bounds.Min.X + bounds.Dx()/2,
		Y: bounds.Min.Y + bounds.Dy()/2,
	}

	h.exitModeLocked()

	go func() {
		if h.actionService == nil {
			return
		}

		err := h.actionService.MoveCursorToPointAndWait(h.ctx, center, true)
		if err != nil {
			h.logger.Error("Failed to move cursor to selected monitor",
				zap.Error(err),
			)
		}
	}()
}

func (h *Handler) cleanupMonitorSelectMode() {
	err := h.hideMonitorSelectLocked()
	if err != nil && !derrors.IsNotSupported(err) {
		h.logger.Debug("Failed to hide monitor_select overlay", zap.Error(err))
	}

	h.monitorSelect = nil
}

func (h *Handler) discoverMonitorsForSelection() ([]monitorSelectTarget, image.Rectangle, error) {
	if h.system == nil {
		return nil, image.Rectangle{}, derrors.New(
			derrors.CodeNotSupported,
			"system integration unavailable",
		)
	}

	names, err := h.system.ScreenNames(h.ctx)
	if err != nil {
		return nil, image.Rectangle{}, err
	}

	currentBounds, err := h.system.ScreenBounds(h.ctx)
	if err != nil {
		return nil, image.Rectangle{}, err
	}

	monitors := make([]monitorSelectTarget, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		bounds, _, boundsErr := h.system.ScreenBoundsByName(h.ctx, name)
		if boundsErr != nil {
			if derrors.IsNotSupported(boundsErr) {
				return nil, image.Rectangle{}, boundsErr
			}

			h.logger.Debug("Skipping monitor with unreadable bounds",
				zap.String("monitor", name),
				zap.Error(boundsErr),
			)

			continue
		}

		monitors = append(monitors, monitorSelectTarget{
			Name:   name,
			Bounds: bounds,
		})
	}

	return monitors, currentBounds, nil
}

func (h *Handler) reportMonitorSelectNotSupported() {
	h.logger.Info("monitor_select is not supported on this platform")

	if h.system != nil {
		h.system.ShowNotification("neru monitor_select", "Not supported on this platform")
	}
}
