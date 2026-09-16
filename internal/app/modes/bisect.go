package modes

import (
	"context"
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentbisect "github.com/y3owk1n/neru/internal/app/components/bisect"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/bisect"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// bisectBundleTimeout bounds the one focused-app lookup an activation makes
// for the per-app scope, so a slow accessibility answer cannot hold the
// activation.
const bisectBundleTimeout = time.Second

// startBisect activates bisect mode. The region starts as the capture area
// the configuration names, or the one the activation's --capture-scope
// overrides it with; the cursor goes to its center and the region is handed
// over as the frame that takes the previous mode's drawing off the screen.
func (h *handlerState) startBisect(activation modecmd.Activation) {
	isRefresh := h.appState.CurrentMode() == domain.ModeBisect

	_, activated := h.activateModeBase(
		domain.ModeNameBisect,
		h.config.Bisect.Enabled,
		action.TypeMoveMouse,
		"",
	)
	if !activated {
		if isRefresh {
			h.exitMode()
		}

		return
	}

	if isRefresh {
		h.stopIndicatorPolling()
	} else {
		// Coming from another mode, the keyboard is handed over rather than
		// given back: exit now, release at return if nothing is entered.
		defer h.exitModeForTransition()()
	}

	screen := h.bisectScreen()
	h.setScreenBounds(screen)

	scope := h.bisectScope(activation)

	var cursorShouldFollow bool
	if isRefresh && activation.CursorFollowSelection == nil && h.bisect != nil &&
		h.bisect.Context != nil {
		cursorShouldFollow = h.bisect.Context.CursorFollowSelection()
	} else {
		cursorShouldFollow = resolveCursorFollowSelection(activation.CursorFollowSelection)
	}

	// The start region is global like the screen; the overlay draws in the
	// screen's own space.
	h.initializeBisectRegion(h.bisectStart(screen, scope).Sub(screen.Min))
	h.bisect.Context.SetCursorFollowSelection(cursorShouldFollow)

	// The mode is entered before the first frame is built, so the quadrant
	// labels come from the keymap settled for bisect and the focused app.
	if !isRefresh {
		h.enterMode(domain.ModeBisect)
	}

	// One draw at activation: the frame comes up as the transition, and the
	// cursor is placed without the redraw a cut pays.
	center := h.markBisectSelection()
	h.showFrame(h.bisectFrame(), "show bisect overlay")
	h.placeBisectCursor(center, "Failed to move cursor to the bisect region center")

	h.logger.Info("Bisect mode activated", zap.String("scope", scope))

	h.startIndicatorPolling(domain.ModeBisect)
}

// bisectScope is the region the session starts from: the configuration's,
// shadowed by the focused application's [[bisect.app_configs]] entry, and
// then by the activation's --capture-scope. The application is asked once,
// under a short bound, the way hints asks at its activation; a lookup that
// fails leaves the configured scope in force.
func (h *handlerState) bisectScope(activation modecmd.Activation) string {
	if activation.CaptureScope != nil {
		return *activation.CaptureScope
	}

	if !h.config.Bisect.HasAppCaptureScopeOverrides() {
		return h.config.Bisect.CaptureScope
	}

	bundleCtx, bundleCancel := context.WithTimeout(h.ctx, bisectBundleTimeout)
	bundleID, bundleIDErr := h.actionService.FocusedAppBundleID(bundleCtx)

	bundleCancel()

	if bundleIDErr != nil {
		h.logger.Debug("Failed to get the focused app for the bisect scope", zap.Error(bundleIDErr))

		return h.config.Bisect.CaptureScope
	}

	return h.config.Bisect.CaptureScopeForApp(bundleID)
}

// bisectScreen is the display the session is drawn on: the active screen,
// or the bounds the handler already holds when the platform cannot say.
func (h *handlerState) bisectScreen() image.Rectangle {
	if h.system == nil {
		return h.screenBounds
	}

	bounds, err := h.system.ScreenBounds(h.ctx)
	if err != nil {
		if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for bisect", zap.Error(err))
		}

		return h.screenBounds
	}

	return bounds
}

// bisectStart is the region the session begins with, in global coordinates:
// the focused window when the scope asks for it and one is focused, and the
// screen otherwise. A window that runs off the screen is cut to it, since
// the frame is drawn in that screen's space.
func (h *handlerState) bisectStart(screen image.Rectangle, scope string) image.Rectangle {
	if scope != domain.CaptureScopeWindow || h.system == nil {
		return screen
	}

	window, focused, err := h.system.FocusedWindowBounds(h.ctx)
	if err != nil {
		// Warned at every code, CodeNotSupported included: the user asked for
		// a window and is getting the screen.
		h.logger.Warn("Failed to get the focused window for bisect; using the screen",
			zap.Error(err))

		return screen
	}

	if !focused || window.Empty() {
		h.logger.Debug("No focused window for bisect; using the screen")

		return screen
	}

	clipped := window.Intersect(screen)
	if clipped.Empty() {
		return screen
	}

	return clipped
}

// initializeBisectRegion builds a fresh session over start, in screen-local
// coordinates.
func (h *handlerState) initializeBisectRegion(start image.Rectangle) {
	if h.bisect == nil {
		h.bisect = &components.BisectComponent{Context: &componentbisect.Context{}}
	}

	h.bisect.Region = bisect.NewRegion(start)
}

// bisectCut keeps the half or quadrant cut names and settles the cursor on
// the center of what is left.
func (h *handlerState) bisectCut(cut bisect.Cut) {
	if h.bisect == nil || h.bisect.Region == nil {
		h.logger.Warn("Bisect region is nil - ignoring press")

		return
	}

	if !h.bisect.Region.Apply(cut) {
		h.logger.Debug("Bisect cut changed nothing", zap.String("cut", cut.String()))

		return
	}

	h.logger.Debug("Bisect cut",
		zap.String("cut", cut.String()),
		zap.Int("depth", h.bisect.Region.Depth()))

	h.settleBisect("Failed to move cursor after bisect cut")
}

// settleBisect remembers the region's center as the selection, redraws the
// region with the pointer stand-in riding it, and brings whichever pointer
// stands in for the selection onto the center: the real cursor when it
// follows, the virtual one otherwise.
func (h *handlerState) settleBisect(moveFailureMessage string) {
	if h.bisect == nil || h.bisect.Region == nil {
		return
	}

	center := h.markBisectSelection()
	h.updateBisectOverlay()
	h.placeBisectCursor(center, moveFailureMessage)
}

// markBisectSelection records the region's center as the selection and
// answers it in global coordinates.
func (h *handlerState) markBisectSelection() image.Point {
	center := geometry.ConvertToAbsoluteCoordinates(h.bisect.Region.Center(), h.screenBounds)

	if h.bisect.Context != nil {
		h.bisect.Context.SetSelectionPoint(center)
	}

	return center
}

// placeBisectCursor moves the real cursor onto center when it follows the
// selection; when held back, the pointer stand-in on the frame marks it.
func (h *handlerState) placeBisectCursor(center image.Point, moveFailureMessage string) {
	if h.bisect.Context != nil && !h.bisect.Context.CursorFollowSelection() {
		return
	}

	err := h.actionService.MoveCursorToPoint(h.ctx, center)
	if err != nil {
		h.logger.Error(moveFailureMessage, zap.Error(err))
	}
}

// updateBisectOverlay redraws the region on the overlay it is already on.
func (h *handlerState) updateBisectOverlay() {
	if h.bisect == nil || h.bisect.Region == nil {
		return
	}

	h.redrawFrame(h.bisectFrame(), "update bisect overlay")
}

// bisectFrame describes what should be on screen: the region divided in
// four, each quadrant labeled with the key that keeps it.
func (h *handlerState) bisectFrame() ports.BisectFrame {
	if h.bisect == nil || h.bisect.Region == nil {
		return ports.BisectFrame{}
	}

	return ports.BisectFrame{
		Bounds:  h.bisect.Region.Bounds(),
		Depth:   h.bisect.Region.Depth(),
		Keys:    bisectQuadrantKeys(h.settledKeymap().Bindings()),
		Pointer: h.bisectPointer(),
	}
}

// bisectQuadrantKeys reads the bindings in force for the key bound to each
// quadrant cut, in reading order, so the cells are labeled with whatever
// the user actually presses, per-app overrides included. An unbound quadrant
// is a space, drawn as an unlabelled cell. When several keys reach one
// quadrant the alphabetically first wins, so the answer is stable.
func bisectQuadrantKeys(bindings []configpkg.Binding) string {
	quadrants := [4]bisect.Cut{
		bisect.CutUpLeft,
		bisect.CutUpRight,
		bisect.CutDownLeft,
		bisect.CutDownRight,
	}
	labels := [4]string{" ", " ", " ", " "}

	for _, binding := range bindings {
		// A quadrant cell shows one character, so a named key has no place in it.
		if len(binding.Steps) != 1 || utf8.RuneCountInString(binding.Key) != 1 {
			continue
		}

		key := binding.Key

		cut, ok := bisectCutOfBinding(binding.Steps[0])
		if !ok {
			continue
		}

		for index, quadrant := range quadrants {
			if cut == quadrant && (labels[index] == " " || key < labels[index]) {
				labels[index] = key
			}
		}
	}

	return strings.Join(labels[:], "")
}

// bisectCutOfBinding reads the cut out of a binding step of the form
// "action bisect --direction=<cut>" or "action bisect --direction <cut>".
func bisectCutOfBinding(step string) (bisect.Cut, bool) {
	fields := strings.Fields(step)

	const prefixLen = 2 // "action bisect"
	if len(fields) < prefixLen+1 || fields[0] != "action" ||
		fields[1] != string(action.NameBisect) {
		return 0, false
	}

	for index := prefixLen; index < len(fields); index++ {
		value, joined := strings.CutPrefix(fields[index], "--direction=")
		if !joined {
			if fields[index] != "--direction" || index+1 >= len(fields) {
				continue
			}

			value = fields[index+1]
		}

		cut, err := bisect.ParseCut(value)
		if err != nil {
			return 0, false
		}

		return cut, true
	}

	return 0, false
}

// refreshBisectForMonitorMove remaps the region onto the known target screen
// and shows the overlay there.
func (h *handlerState) refreshBisectForMonitorMove(targetBounds image.Rectangle) {
	h.setScreenBounds(targetBounds)
	h.remapBisect(geometry.NormalizeToLocalCoordinates(targetBounds))
	h.showFrame(h.bisectFrame(), "refresh bisect after monitor move")
}

// refreshBisectForScreenChange remaps the region onto the display as it now
// is and hands it over as a transition. The caller holds h.mu (ADR 0004).
func (h *handlerState) refreshBisectForScreenChange() {
	h.setScreenBounds(h.bisectScreen())
	h.remapBisect(geometry.NormalizeToLocalCoordinates(h.screenBounds))

	if h.bisect != nil && h.bisect.Region != nil {
		h.showFrame(h.bisectFrame(), "refresh bisect after screen change")
	}
}

// remapBisect carries the session onto new screen-local bounds: the region
// and its history scale proportionally, and the stale selection is dropped.
// A session with no region is built fresh over the whole screen.
func (h *handlerState) remapBisect(normalizedBounds image.Rectangle) {
	if h.bisect != nil && h.bisect.Region != nil {
		h.bisect.Region.RemapToNewBounds(normalizedBounds)
	} else {
		h.initializeBisectRegion(normalizedBounds)
	}

	if h.bisect != nil && h.bisect.Context != nil {
		h.bisect.Context.ClearSelectionPoint()
	}
}

// cleanupBisectMode handles cleanup for bisect mode.
func (h *handlerState) cleanupBisectMode() {
	if h.bisect != nil {
		if h.bisect.Context != nil {
			h.bisect.Context.Reset()
		}

		h.bisect.Region = nil

		// Take the pointer stand-in down before the frame is cleared, so its
		// removal does not depend on what the overlay clear happens to reset.
		h.updateGridPointer(domain.ModeBisect, ports.GridPointer{})
	}

	// Stop the indicator poller before common cleanup takes the frame off
	// the screen: a tick landing after the clear would put one back on it.
	h.stopIndicatorPolling()
	h.stopHeldRepeat()
}
