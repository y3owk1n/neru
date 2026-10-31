package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// refreshHintsForScreenChange regenerates the hint collection against the
// display as it now is, with the filters the session was activated with, and
// hands the result over as a transition onto that display.
//
// It reports whether the labels reached the screen. Every path that says no has
// left the mode, because hints pointing at a display that is gone are worse
// than none.
//
// The caller holds h.mu (HintsMode.RefreshForScreenChange, dispatched by
// Handler.RefreshActiveModeForScreenChange), so the mode this belongs to is
// still the active one and there is nothing here to re-check (ADR 0004).
func (h *handlerState) refreshHintsForScreenChange(ctx context.Context) bool {
	// Re-read screen bounds under the lock so the onUpdate callback
	// uses coordinates that match the resized overlay.
	if h.system != nil {
		b, err := h.system.ScreenBounds(ctx)
		if err == nil {
			h.setScreenBounds(b)
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to refresh screen bounds after screen change", zap.Error(err))
		}
	}

	// Escape any active IME search session before refreshing hints on the new
	// screen. The old IME session is bound to the previous screen and loses
	// focus during the space transition, causing subsequent keystrokes to be
	// forwarded to the frontmost app instead.
	if h.hints.Context.SearchActive() {
		h.cancelHintSearch()
	}

	// Nothing can scan supplied targets again, and their coordinates describe
	// the display as it was.
	if h.hints.Context.TargetsSupplied() {
		h.logger.Debug("Screen changed under supplied hint targets; exiting hints mode")
		h.exitMode()

		return false
	}

	// Get the filters the mode was activated with from the context
	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	strategyOverride := h.hints.Context.StrategyOverride()
	captureScopeOverride := h.hints.Context.CaptureScopeOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
	splitWordOverride := h.hints.Context.SplitWord()

	// Generate hints with filters preserved. SetHints below performs the
	// single redraw after active-screen filtering.
	//
	// The walk is given the same HintTimeout budget the activation and
	// monitor-move paths give it. The context a screen change arrives with is
	// the application's own and carries no deadline, and this walk runs under
	// h.mu, so a tree that never answers would hold the lock, and with it every
	// keystroke, for as long as it took.
	//
	// The traversal reads the context as it descends, so this bounds the
	// expensive part rather than only the entry to it.
	generateCtx, cancelGenerate := context.WithTimeout(ctx, HintTimeout)
	defer cancelGenerate()

	domainHints, showHintsErr := h.hintService.GenerateHints(
		generateCtx,
		filterRoles,
		filterTextContains,
		"",
		strategyOverride,
		captureScopeOverride,
		labelDirectionOverride,
		splitWordOverride,
	)
	if showHintsErr != nil {
		h.logger.Warn("Failed to refresh hints after screen change", zap.Error(showHintsErr))
		h.exitMode()

		return false
	}

	if len(domainHints) == 0 {
		h.logger.Debug("No hints after screen change refresh")
		h.exitMode()

		return false
	}

	allHints := domainHints

	filtered := filterHintsForScreen(allHints, h.screenBounds)
	if len(filtered) == 0 {
		h.logger.Debug("No hints on active screen after filter; skipping refresh")
		h.exitMode()

		return false
	}

	// The screen moved under the overlay, so the next draw is a transition
	// onto the new one rather than a repaint of the old. It has to be resized
	// to the new display, shown and switched, and that whole sequence belongs
	// to the overlay. As on the activation and monitor-move paths, the flag is
	// cleared immediately before SetHints, and SetHints draws that
	// transition.
	h.hintsFrameOnScreen = false

	setHintsErr := h.hints.Context.SetHints(
		domainHint.NewCollection(filtered),
	)
	if setHintsErr != nil {
		// The flag was cleared above, so the overlay is neither sized for the
		// new display nor showing a collection that belongs to it. Leaving the
		// mode running would leave the old screen's labels on screen at the old
		// size. Exiting is what the three failure paths above already do.
		h.logger.Warn("Failed to refresh hints for screen change", zap.Error(setHintsErr))
		h.exitMode()

		return false
	}

	return true
}

// refreshGridForScreenChange regenerates the grid against the display as it
// now is and hands it over as a transition onto that display. The user's
// current input is reset because old cell coordinates are invalid on the new
// screen.
//
// It reports whether the grid reached the screen. Only a draw the overlay
// refused says no.
//
// The caller holds h.mu (GridMode.RefreshForScreenChange, dispatched by
// Handler.RefreshActiveModeForScreenChange), so the mode this belongs to is
// still the active one and there is nothing here to re-check (ADR 0004).
func (h *handlerState) refreshGridForScreenChange() bool {
	// Regenerate the grid with updated screen bounds.
	// createGridInstance also updates h.screenBounds and sets the grid on the context.
	gridInstance := h.createGridInstance(h.grid.Context.CaptureScope())

	if h.grid.Manager != nil {
		// Sync the Manager's internal grid reference so subsequent key presses
		// use the new grid's geometry for cell matching (fixes stale-bounds bug).
		h.grid.Manager.UpdateGrid(gridInstance)

		// Reset input state because old cell coordinates/bounds are invalid on
		// the new screen, and any in-progress subgrid selection would reference
		// a stale cell.
		h.grid.Manager.Reset()
	}

	// Clear the stale selection, since old coordinates are invalid on the new
	// screen.
	h.grid.Context.ClearSelectionPoint()

	// The screen changed under the overlay, so this is a transition onto the
	// new one, and it is resized and shown as well as redrawn.
	if !h.showFrame(
		ports.GridFrame{Grid: gridInstance},
		"refresh grid after screen change",
	) {
		return false
	}

	// The selection was cleared, so this hides the virtual pointer. The grid
	// draw may already clear cursorIndicatorVisible through NeruClearOverlay,
	// but hiding it here does not depend on that.
	h.refreshGridVirtualPointer()

	return true
}

// refreshRecursiveGridForScreenChange remaps the recursive-grid manager's
// bounds to the display as it now is, preserving the user's current depth, and
// hands the result over as a transition onto that display.
//
// The caller holds h.mu (RecursiveGridMode.RefreshForScreenChange, dispatched
// by Handler.RefreshActiveModeForScreenChange), so the mode this belongs to is
// still the active one and there is nothing here to re-check (ADR 0004).
func (h *handlerState) refreshRecursiveGridForScreenChange() {
	// Re-read screen bounds under the lock so the overlay uses coordinates
	// that match the resized window.
	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			h.setScreenBounds(b)
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to refresh screen bounds for recursive grid", zap.Error(err))
		}
	}

	// The session's kind of region, screen or window, read from the display
	// as it now is.
	scope := ""
	if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
		scope = h.recursiveGrid.Context.CaptureScope()
	}

	region := h.captureStart(domain.ModeNameRecursiveGrid, h.screenBounds, scope).
		Sub(h.screenBounds.Min)

	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		// Proportionally remap all bounds (history + currentBounds) so the
		// user's zoomed-in region maps to the equivalent area on the new screen.
		h.recursiveGrid.Manager.CurrentGrid().RemapToNewBounds(region)
		h.recursiveGrid.Manager.CurrentGrid().SetMinSize(h.recursiveGridMinSize())
	} else {
		// With no manager, fall back to full initialization.
		h.initializeRecursiveGridManager(region)
	}

	// Clear the stale selection, since old coordinates are invalid on the new
	// screen.
	if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.ClearSelectionPoint()
	}

	// The screen moved under the overlay, so this is a transition onto the new
	// one. It is resized, shown and switched as well as drawn, and the overlay
	// owns that sequence. Handing over a redraw instead would leave the surface
	// sized for the display just left.
	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		h.showFrame(h.recursiveGridFrame(), "refresh recursive grid after screen change")
	}

	h.refreshRecursiveGridVirtualPointer()
}

// RefreshActiveModeForThemeChange puts whichever mode is on screen back on it
// in the colors the system just switched to. The overlay has already
// re-resolved every Style for itself, so all that is left is the mode drawing
// what it already holds.
//
// This is the whole dispatch and it runs under one hold of h.mu. The mode is
// read once, selected once and called once, so it cannot change between being
// chosen and being used and no implementation re-checks it (ADR 0004). Scroll
// and idle answer by not carrying the axis, since scroll draws nothing of its
// own and idle has no entry in the mode map at all. That is the same nothing
// the app layer's empty switch arms used to say, now with a debug line naming
// what declined.
//
// It reports whether the active mode redrew, so a caller can tell a theme
// change that reached the screen from one that found nothing to repaint.
func (h *Handler) RefreshActiveModeForThemeChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	refresher, ok := activeModeEffect[themeRefresher](&h.handlerState, extensionThemeRefresh)
	if !ok {
		return false
	}

	return refresher.RefreshForThemeChange()
}

// RefreshActiveModeForScreenChange puts whichever mode is on screen back onto
// the display as it now is, after a monitor is plugged in or unplugged, a
// resolution changes, or a laptop wakes to a different arrangement. It reports
// whether the overlay still needs a resize of its own afterwards.
//
// This is the whole dispatch and it runs under one hold of h.mu. The mode is
// read once, selected once and called once, so it cannot change between being
// chosen and being used and no implementation re-checks it (ADR 0004). The
// unlocked snapshot the app layer used to take, and the three per-mode
// re-checks that guarded the window it opened, went with it.
//
// Idle answers first, and answers that nothing is owed. With no mode open there
// is nothing drawn for the display that changed, and resizing the overlay is
// what brings it up, so the display change a user has most often, with nothing
// on screen, leaves the screen alone. Scroll and the monitor picker carry the
// axis by not implementing it, because neither holds a drawing built for the
// bounds that just changed, and the overlay is resized for them the way it
// always was. A mode that rebuilt nothing, because it is switched off in
// configuration or has no session state to rebuild from, is resized for the
// same way.
//
// It reports whether the overlay still needs a resize of its own, never
// whether a refresh succeeded. A refresh that found nothing to draw and left
// the mode took the overlay off the display it was on, and resizing it
// afterwards would bring it back up empty.
func (h *Handler) RefreshActiveModeForScreenChange(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() == domain.ModeIdle {
		return false
	}

	refresher, ok := activeModeEffect[screenRefresher](&h.handlerState, extensionScreenRefresh)
	if !ok {
		return true
	}

	overlayResized := refresher.RefreshForScreenChange(ctx)

	return !overlayResized
}
