package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/ports"
)

// CurrentSelectionPoint returns the active selection point for the current
// mode, if any. A mode that tracks no selection reports none.
func (h *Handler) CurrentSelectionPoint() (image.Point, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tracker, ok := activeModeExtension[selectionTracker](&h.handlerState)
	if !ok {
		return image.Point{}, false
	}

	return tracker.SelectionPoint()
}

// ClearCurrentSelectionPoint removes the active selection point for the
// current mode, reporting false when the active mode tracks no selection.
func (h *Handler) ClearCurrentSelectionPoint() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	tracker, ok := activeModeExtension[selectionTracker](&h.handlerState)
	if !ok {
		return false
	}

	return tracker.ClearSelectionPoint()
}

// cursorFollowContext is the part of a mode's context that carries the
// session's cursor-follow-selection preference. Hints, grid, and recursive grid
// each have their own context type; this is the shape they share, so a mode can
// read and write the preference without the three implementations repeating
// how it is set.
type cursorFollowContext interface {
	CursorFollowSelection() bool
	SetCursorFollowSelection(cursorFollowSelection bool)
	ToggleCursorFollowSelection() bool
}

// applyCursorFollow writes the preference desired names onto modeContext, or
// toggles it when desired is nil, and reports the value it settled on. What a
// mode owes the change afterwards is the mode's own business.
func applyCursorFollow(modeContext cursorFollowContext, desired *bool) bool {
	if desired == nil {
		return modeContext.ToggleCursorFollowSelection()
	}

	modeContext.SetCursorFollowSelection(*desired)

	return *desired
}

// CursorFollowSelection reports whether the active mode's session follows the
// selection with the real cursor. The second result is false when no mode that
// carries the preference is active, which is the same condition under which
// toggling it is refused.
func (h *Handler) CursorFollowSelection() (bool, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	follower, ok := activeModeExtension[cursorFollowSelector](&h.handlerState)
	if !ok {
		return false, false
	}

	return follower.CursorFollowSelection()
}

// ToggleCursorFollowSelection toggles cursor-follow-selection for the active mode.
func (h *Handler) ToggleCursorFollowSelection() (bool, bool) {
	return h.applyCursorFollowSelection(nil)
}

// SetCursorFollowSelection turns cursor-follow-selection on or off for the
// active mode, so a caller that knows which state it wants can converge on it
// rather than flipping whatever is there.
func (h *Handler) SetCursorFollowSelection(enabled bool) (bool, bool) {
	return h.applyCursorFollowSelection(&enabled)
}

// applyCursorFollowSelection sets the preference to desired, or toggles it when
// desired is nil, and reports the resulting value.
//
// Setting the preference to the value it already holds still runs whatever the
// mode owes the change. That is what makes the setter idempotent in the way a
// caller needs: "on" always ends with the cursor on the selection, whether or
// not it was already following.
func (h *Handler) applyCursorFollowSelection(desired *bool) (bool, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	follower, ok := activeModeExtension[cursorFollowSelector](&h.handlerState)
	if !ok {
		return false, false
	}

	return follower.ApplyCursorFollowSelection(desired)
}

// moveCursorToSelection moves the real cursor onto the mode's stored
// selection point when the mode is following the selection. Turning the
// preference off leaves the cursor where it is.
func (h *handlerState) moveCursorToSelection(
	enabled bool,
	selectionPoint func() (image.Point, bool),
) {
	if !enabled || h.actionService == nil {
		return
	}

	target, ok := selectionPoint()
	if !ok {
		return
	}

	moveCursorErr := h.actionService.MoveCursorToPoint(h.ctx, target)
	if moveCursorErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
	}
}

// refreshGridVirtualPointer puts the pointer stand-in where grid mode's
// selection is, or takes it off when the cursor follows the selection itself
// and there is nothing to stand in for.
//
// It is the call for the keystrokes that move the pointer and nothing else. The
// one that opens a subgrid moves both, and carries the value below on the open
// instead (#1492).
func (h *handlerState) refreshGridVirtualPointer() {
	if h.grid == nil || h.grid.Context == nil {
		return
	}

	h.updateGridPointer(domain.ModeGrid, h.gridPointer())
}

// gridPointer is the pointer grid mode should be showing. Like
// recursiveGridPointer it is read twice — as a call of its own, and as part of
// the subgrid open that repaints the surface it stands on.
func (h *handlerState) gridPointer() ports.GridPointer {
	if h.grid == nil || h.grid.Context == nil {
		return ports.GridPointer{}
	}

	return selectionPointer(
		h.grid.Context.SelectionPoint,
		h.grid.Context.CursorFollowSelection(),
		h.screenBounds,
	)
}

// refreshRecursiveGridVirtualPointer does the same for recursive-grid mode.
func (h *handlerState) refreshRecursiveGridVirtualPointer() {
	h.updateGridPointer(domain.ModeRecursiveGrid, h.recursiveGridPointer())
}

// recursiveGridPointer is the pointer recursive-grid mode should be showing.
// It is read twice — as a call of its own, and as part of the frame the
// surface is redrawn with — because the backend paints the pointer in the same
// pass as the cells.
func (h *handlerState) recursiveGridPointer() ports.GridPointer {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
		return ports.GridPointer{}
	}

	return selectionPointer(
		h.recursiveGrid.Context.SelectionPoint,
		h.recursiveGrid.Context.CursorFollowSelection(),
		h.screenBounds,
	)
}

// selectionPointer turns a mode's selection into the pointer the overlay
// should draw: visible only when there is a selection and the real cursor is
// not already sitting on it, in the overlay's own screen-local space.
func selectionPointer(
	selection func() (image.Point, bool),
	cursorFollowsSelection bool,
	screenBounds image.Rectangle,
) ports.GridPointer {
	point, ok := selection()
	if !ok || cursorFollowsSelection {
		return ports.GridPointer{}
	}

	return ports.GridPointer{
		Visible:  true,
		Position: geometry.ConvertToLocalCoordinates(point, screenBounds),
	}
}
