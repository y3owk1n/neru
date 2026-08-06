package modes

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	indicatorPollInterval = 16 * time.Millisecond
	indicatorPollTimeout  = 100 * time.Millisecond
)

// startIndicatorPolling starts a goroutine that polls the cursor position and
// updates the mode indicator and sticky modifiers indicator overlays.
// It is shared by all navigation modes.
func (h *handlerState) startIndicatorPolling(mode domain.Mode) {
	// If polling is already active, do not start another goroutine.
	if h.indicatorTicker != nil || h.indicatorStopCh != nil {
		return
	}

	// All callers hold h.mu, so state methods are callable directly.
	if h.config == nil || !h.shouldPollCursorOverlays(mode) {
		return
	}
	// Disable exclusive keyboard so scroll events pass through to applications
	// when indicator overlays are shown, but only if uinput scroll is active.
	// Skip when an evdev keyboard grab owns the keyboard: there the overlay
	// must stay keyboard-passive, and the evdev path already keeps it that way.
	setOverlayKeyboardCapture(h, false)

	// Size the indicators before the goroutine draws. Grid and recursive-grid
	// manage their own windows and skip the manager's resize, so these could
	// still be sized for a different monitor.
	h.resizeIndicators()

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	ticker := time.NewTicker(indicatorPollInterval)
	h.indicatorStopCh = stopCh
	h.indicatorDoneCh = doneCh

	h.indicatorTicker = ticker

	go h.outer.runIndicatorPolling(stopCh, doneCh, ticker)
}

// runIndicatorPolling is the polling loop. It exits when stopCh closes.
func (h *Handler) runIndicatorPolling(
	stopCh chan struct{},
	doneCh chan struct{},
	ticker *time.Ticker,
) {
	defer close(doneCh)

	ctx, cancel := context.WithCancel(h.ctx)
	defer cancel()

	// Cancel the context as soon as stop is signaled so a hung call unblocks.
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Re-check stopCh before doing any work to shrink the window
			// where a draw can be dispatched after stop is signaled.
			select {
			case <-stopCh:
				return
			default:
			}

			h.pollIndicatorsOnce(ctx)
		}
	}
}

// indicatorSnapshot is what one tick decides to draw, read under h.mu.
type indicatorSnapshot struct {
	showModeIndicator  bool
	stickyEnabled      bool
	stickyActive       bool
	stickySymbols      string
	showVirtualPointer bool
	stickyPoint        image.Point
}

// indicatorTickPlan is what one tick decided to do. It is computed under h.mu
// and executed by pollIndicatorsOnce after the lock is released, so a draw is
// never made while the handler's lock is held.
type indicatorTickPlan struct {
	// resizeIndicators means the screen bounds changed under the cursor:
	// resize the indicator overlays instead of drawing, and let the next tick
	// work from the updated bounds.
	resizeIndicators bool

	snap indicatorSnapshot
}

// pollIndicatorsOnce runs one tick: read the cursor, compute a plan under the
// lock, then execute it with the lock released. Skipped entirely when the lock
// is contended.
func (h *Handler) pollIndicatorsOnce(ctx context.Context) {
	reqCtx, reqCancel := context.WithTimeout(ctx, indicatorPollTimeout)
	cursorX, cursorY, err := h.modeIndicatorService.GetCursorPosition(reqCtx)

	reqCancel()

	if err != nil {
		return
	}

	plan, ok := h.planIndicatorTick(ctx, cursorX, cursorY)
	if !ok {
		return
	}

	if plan.resizeIndicators {
		h.resizeIndicators()

		return
	}

	h.drawIndicators(plan.snap, cursorX, cursorY)

	// Re-hide the system cursor if macOS revealed it (e.g. right-click menu,
	// Mission Control). Rate-limited to ~2 Hz.
	h.rehideSystemCursor()

	// Flush atomically so no intermediate state shows between overlay updates.
	if h.overlayPort != nil {
		h.overlayPort.Flush()
	}
}

// planIndicatorTick computes what this tick should do. It takes and releases
// h.mu itself; the caller executes the returned plan with the lock free.
//
// TryLock, not Lock: stopIndicatorPolling runs while h.mu is held and blocks
// on indicatorDoneCh, so waiting here would deadlock. A contended tick is just
// skipped, reported as ok == false.
func (h *Handler) planIndicatorTick(
	ctx context.Context,
	cursorX, cursorY int,
) (indicatorTickPlan, bool) {
	if !h.mu.TryLock() {
		return indicatorTickPlan{}, false
	}
	defer h.mu.Unlock()

	if !image.Pt(cursorX, cursorY).In(h.screenBounds) && h.system != nil &&
		h.adoptChangedScreenBounds(ctx) {
		return indicatorTickPlan{resizeIndicators: true}, true
	}

	return indicatorTickPlan{snap: h.snapshotIndicators(cursorX, cursorY)}, true
}

// snapshotIndicators reads everything one tick's draw needs. Caller must hold
// h.mu.
func (h *handlerState) snapshotIndicators(cursorX, cursorY int) indicatorSnapshot {
	snap := indicatorSnapshot{
		showModeIndicator:  h.modeIndicatorEnabled(h.appState.CurrentMode()),
		stickyEnabled:      h.stickyModifiersEnabled(),
		showVirtualPointer: h.shouldShowCursorFollowingVirtualPointer(),
	}

	// Active means there is something to show: modifiers held, and symbols to
	// draw for them. Deciding it here keeps the draw reading one flag.
	if mods := h.stickyModifiers(); mods != 0 {
		snap.stickySymbols = stickyindicator.ModifierSymbolsString(mods)
		snap.stickyActive = snap.stickySymbols != ""
	}

	snap.stickyPoint = h.stickyIndicatorAnchor(image.Pt(cursorX, cursorY))

	return snap
}

// adoptChangedScreenBounds re-reads the screen bounds and records them when
// they changed, reporting whether they did. Caller must hold h.mu; the overlay
// resize that must follow happens after the lock is released.
func (h *handlerState) adoptChangedScreenBounds(ctx context.Context) bool {
	boundsCtx, boundsCancel := context.WithTimeout(ctx, indicatorPollTimeout)
	newBounds, boundsErr := h.system.ScreenBounds(boundsCtx)

	boundsCancel()

	if boundsErr != nil || newBounds == h.screenBounds {
		return false
	}

	h.setScreenBounds(newBounds)

	return true
}

// resizeIndicators sizes each indicator to the active screen.
//
// The polling goroutine calls it with h.mu released, from the plan a tick
// computed. startIndicatorPolling calls it once before the goroutine starts,
// under the lock every mode activation already holds, as it always has: the
// resize dispatches to the main queue on macOS and is a no-op where an
// indicator has no window of its own.
func (h *handlerState) resizeIndicators() {
	h.modeIndicatorService.ResizeToActiveScreen()
	h.stickyIndicatorService.ResizeToActiveScreen()
	h.virtualPointerService.ResizeToActiveScreen()
}

// drawIndicators shows, draws or hides each indicator from one snapshot. It
// runs with h.mu released — everything it needs is in the snapshot. The
// indicators take absolute global coordinates; the backend clamps them to the
// active display.
func (h *Handler) drawIndicators(snap indicatorSnapshot, cursorX, cursorY int) {
	if snap.showModeIndicator {
		h.modeIndicatorService.Show()
		h.modeIndicatorService.UpdateIndicatorPosition(cursorX, cursorY)
	} else {
		h.modeIndicatorService.Hide()
	}

	if snap.stickyEnabled {
		if snap.stickyActive {
			h.stickyIndicatorService.Show()
			h.stickyIndicatorService.UpdateIndicatorPosition(
				snap.stickyPoint.X,
				snap.stickyPoint.Y,
				snap.stickySymbols,
			)
		} else {
			h.stickyIndicatorService.Hide()
		}
	}

	if snap.showVirtualPointer {
		h.virtualPointerService.Show()
		h.virtualPointerService.UpdateIndicatorPosition(cursorX, cursorY)
	} else {
		h.virtualPointerService.Hide()
	}
}

// stopIndicatorPolling stops the indicator polling goroutine and cleans up
// both mode indicator and sticky modifiers indicator overlays.
func (h *handlerState) stopIndicatorPolling() {
	// Restore keyboard capture if uinput scroll was active. Never re-enable it
	// while an evdev keyboard grab owns the keyboard: on wlroots compositors the
	// overlay's exclusive keyboard grab deactivates the focused app's toplevel,
	// so a hints refresh (which stops indicator polling before rescanning) would
	// re-read the wrong focused window and clear the hints.
	setOverlayKeyboardCapture(h, true)

	// Signal stop first.
	if h.indicatorStopCh != nil {
		close(h.indicatorStopCh)
		h.indicatorStopCh = nil
	}
	// Wait for polling goroutine to finish to avoid race conditions where
	// the indicator is drawn after cleanup.
	if h.indicatorDoneCh != nil {
		<-h.indicatorDoneCh
		h.indicatorDoneCh = nil
	}
	// Hide the indicators AFTER the goroutine has fully stopped, so a late
	// draw dispatched by the last tick cannot put one back on screen.
	h.modeIndicatorService.Hide()
	h.stickyIndicatorService.Hide()

	// The virtual pointer stands in for the system cursor rather than for a
	// mode, so it outlives polling whenever the cursor is still hidden.
	// All callers hold h.mu, so state methods are callable directly.
	if !h.shouldShowCursorFollowingVirtualPointer() {
		h.hideCursorFollowingVirtualPointer()
	}
	// Clean up resources after loop has exited.
	if h.indicatorTicker != nil {
		h.indicatorTicker.Stop()
		h.indicatorTicker = nil
	}
}

func (h *handlerState) modeIndicatorEnabled(mode domain.Mode) bool {
	if h.config == nil {
		return false
	}

	switch mode {
	case domain.ModeIdle:
		return false
	case domain.ModeScroll:
		return h.config.ModeIndicator.Scroll.Enabled
	case domain.ModeHints:
		return h.config.ModeIndicator.Hints.Enabled
	case domain.ModeGrid:
		return h.config.ModeIndicator.Grid.Enabled
	case domain.ModeRecursiveGrid:
		return h.config.ModeIndicator.RecursiveGrid.Enabled
	case domain.ModeMonitorSelect:
		return h.config.ModeIndicator.MonitorSelect.Enabled
	default:
		return false
	}
}

func (h *handlerState) stickyIndicatorAnchor(cursorPoint image.Point) image.Point {
	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil || h.grid.Context.CursorFollowSelection() {
			return cursorPoint
		}

		if selectionPoint, ok := h.grid.Context.SelectionPoint(); ok {
			return selectionPoint
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil ||
			h.recursiveGrid.Context.CursorFollowSelection() {
			return cursorPoint
		}

		if selectionPoint, ok := h.recursiveGrid.Context.SelectionPoint(); ok {
			return selectionPoint
		}
	case domain.ModeIdle:
	case domain.ModeHints:
	case domain.ModeMonitorSelect:
	case domain.ModeScroll:
	}

	return cursorPoint
}

// setOverlayKeyboardCapture asks the overlay to hold or release the keyboard.
//
// Only the Linux backends can do this, so it is an optional extension reached
// by type assertion: elsewhere the assertion fails and the call is a no-op,
// which is the right behavior — no other backend's overlay takes the keyboard
// away from the focused application in the first place.
func setOverlayKeyboardCapture(h *handlerState, enabled bool) {
	if !h.allowsOverlayKeyboardPassthrough() {
		return
	}

	controller, ok := overlay.Get().(overlaymanager.KeyboardCaptureController)
	if !ok {
		return
	}

	controller.SetKeyboardCaptureEnabled(enabled)
}
