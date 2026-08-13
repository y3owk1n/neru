package modes

import (
	"context"
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension the monitor picker opts into (extensions.go).
var (
	_ Mode           = (*MonitorSelectMode)(nil)
	_ inputEditor    = (*MonitorSelectMode)(nil)
	_ themeRefresher = (*MonitorSelectMode)(nil)
)

// MonitorSelectMode implements the Mode interface for interactive monitor picking.
type MonitorSelectMode struct {
	baseMode
}

// NewMonitorSelectMode creates a new monitor_select mode implementation.
func NewMonitorSelectMode(handler *handlerState) *MonitorSelectMode {
	return &MonitorSelectMode{
		baseMode: newBaseMode(handler, domain.ModeMonitorSelect, "MonitorSelectMode"),
	}
}

// Activate opens the monitor picker.
func (m *MonitorSelectMode) Activate(activation modecmd.Activation) {
	m.handler.activateMonitorSelectMode(activation)
}

// HandleKey processes a key press within the monitor picker.
func (m *MonitorSelectMode) HandleKey(key string) {
	m.handler.handleMonitorSelectKey(key)
}

// RefreshForMonitorMove puts the picker's panels back up after a move. They
// are placed per display and came down with the frame before the warp, so
// leaving them off would strand the user in a mode with nothing to pick from.
func (m *MonitorSelectMode) RefreshForMonitorMove(_ context.Context, _ image.Rectangle) {
	m.handler.refreshMonitorSelectForMonitorMove()
}

// RefreshForThemeChange draws the picker's panels again so they pick up the
// colors the overlay just re-resolved. They sit on panels of their own rather
// than the shared surface, so nothing about the other modes redrawing brings
// the picker along.
func (m *MonitorSelectMode) RefreshForThemeChange() bool {
	if m.handler.monitorSelect == nil {
		return false
	}

	m.handler.redrawMonitorSelect()

	return true
}

// Exit tears the monitor picker down.
func (m *MonitorSelectMode) Exit() {
	m.handler.cleanupMonitorSelectMode()
}

// ResetInput clears the typed monitor label and puts the highlight back on the
// first target.
func (m *MonitorSelectMode) ResetInput() {
	if m.handler.monitorSelect == nil {
		return
	}

	m.handler.monitorSelect.input = ""
	m.handler.monitorSelect.selectedIndex = 0

	m.handler.redrawMonitorSelect()
}

// Backspace takes back the last character of the monitor label being typed.
func (m *MonitorSelectMode) Backspace() {
	if m.handler.monitorSelect == nil {
		return
	}

	m.handler.monitorSelect.Backspace()

	m.handler.redrawMonitorSelect()
}

func (h *handlerState) activateMonitorSelectMode(_ modecmd.Activation) {
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

	monitors, err := h.discoverMonitorsForSelection()
	if err != nil {
		if derrors.IsNotSupported(err) {
			h.reportMonitorSelectNotSupported()
		} else {
			h.logger.Error("Failed to discover monitors for monitor_select", zap.Error(err))
		}

		return
	}

	session := newMonitorSelectSession(monitors, h.config.MonitorSelect)
	if session == nil {
		if len(monitors) == 1 {
			// Single monitor: auto-confirm immediately so that
			// wait_for_mode_exit --bail chains see a completed selection.
			h.appState.SetModeExitReason(state.ModeExitReasonCompleted)
			h.exitMode()
			h.confirmMonitorSelect(&monitors[0])
		} else {
			h.logger.Debug("Skipping monitor_select activation; no selectable monitors")
		}

		return
	}

	// Coming from another mode, the keyboard is handed over rather than given
	// back: exit now, release at return if nothing is entered. Entering from
	// idle takes the same pair — the exit is a no-op there and the release
	// finds nothing to give back — because a picker that cannot draw returns
	// through the same path either way.
	defer h.exitModeForTransition()()

	h.monitorSelect = session

	// The picker comes up as a Frame, so the overlay owns switching to the
	// mode and drawing the panels. An activation that cannot draw leaves the
	// overlay switched to a mode it never showed, so it is cleared here.
	err = h.showFrameResult(h.monitorSelectFrame(), "show monitor_select overlay")
	if err != nil {
		h.monitorSelect = nil

		h.clearOverlayFrame()

		if derrors.IsNotSupported(err) {
			h.reportMonitorSelectNotSupported()
		}

		return
	}

	h.enterMode(domain.ModeMonitorSelect)
	h.startIndicatorPolling(domain.ModeMonitorSelect)
	h.logger.Info("Monitor select mode activated", zap.Int("targets", len(session.targets)))
}

func (h *handlerState) handleMonitorSelectKey(key string) {
	if h.monitorSelect == nil {
		return
	}

	if target := h.monitorSelect.HandleCharacter(key); target != nil {
		h.confirmMonitorSelect(target)

		return
	}

	h.redrawMonitorSelect()
}

func (h *handlerState) confirmMonitorSelect(target *monitorSelectTarget) {
	if target == nil {
		return
	}

	bounds := target.Bounds
	center := image.Point{
		X: bounds.Min.X + bounds.Dx()/2,
		Y: bounds.Min.Y + bounds.Dy()/2,
	}

	h.appState.SetModeExitReason(state.ModeExitReasonCompleted)
	h.exitMode()

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

// cleanupMonitorSelectMode ends the picking session. The panels come off the
// screen with the frame, in the common cleanup that follows this: they are the
// overlay's to take down, and a mode that took half of it down itself is how
// one used to be left behind.
func (h *handlerState) cleanupMonitorSelectMode() {
	h.monitorSelect = nil
}

func (h *handlerState) discoverMonitorsForSelection() ([]monitorSelectTarget, error) {
	if h.system == nil {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"system integration unavailable",
		)
	}

	names, err := h.system.ScreenNames(h.ctx)
	if err != nil {
		return nil, err
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
				return nil, boundsErr
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

	return monitors, nil
}

func (h *handlerState) reportMonitorSelectNotSupported() {
	h.logger.Info("monitor_select is not supported on this platform")

	if h.system == nil {
		return
	}

	// This runs under h.mu, and showing a notification is a session-bus round
	// trip on Linux — a blocking call the locking contract keeps off the lock.
	// The goroutine touches no handler state, only values read here, and a
	// notification that cannot be shown is reported rather than dropped.
	system, logger, ctx := h.system, h.logger, h.ctx

	go func() {
		err := system.ShowNotification(
			ctx,
			"neru monitor_select",
			"Not supported on this platform",
		)
		if err != nil {
			logger.Warn("Could not notify that monitor_select is unsupported", zap.Error(err))
		}
	}()
}
