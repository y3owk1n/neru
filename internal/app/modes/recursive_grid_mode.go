package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension recursive-grid mode opts into (extensions.go).
var (
	_ Mode                   = (*RecursiveGridMode)(nil)
	_ selectionTracker       = (*RecursiveGridMode)(nil)
	_ cellNavigator          = (*RecursiveGridMode)(nil)
	_ cursorFollowSelector   = (*RecursiveGridMode)(nil)
	_ exitStepReporter       = (*RecursiveGridMode)(nil)
	_ inputEditor            = (*RecursiveGridMode)(nil)
	_ hotkeyOverrideReporter = (*RecursiveGridMode)(nil)
	_ themeRefresher         = (*RecursiveGridMode)(nil)
	_ screenRefresher        = (*RecursiveGridMode)(nil)
)

// RecursiveGridMode implements the Mode interface for recursive-grid navigation.
type RecursiveGridMode struct {
	baseMode
}

// NewRecursiveGridMode creates a new recursive-grid mode instance.
func NewRecursiveGridMode(handler *handlerState) *RecursiveGridMode {
	return &RecursiveGridMode{
		baseMode: newBaseMode(handler, domain.ModeRecursiveGrid, "RecursiveGridMode"),
	}
}

// Activate enters recursive-grid mode with the flags the activation carries.
func (m *RecursiveGridMode) Activate(activation modecmd.Activation) {
	m.handler.activateRecursiveGridModeWithAction(activation)
}

// HandleKey processes a key press within recursive-grid mode.
func (m *RecursiveGridMode) HandleKey(key string) {
	m.handler.handleRecursiveGridKey(key)
}

// RefreshForMonitorMove remaps the zoom history onto the display the cursor
// landed on and hands the result over as a Frame, so the region the user had
// narrowed to survives the move instead of starting over.
func (m *RecursiveGridMode) RefreshForMonitorMove(
	_ context.Context,
	targetBounds image.Rectangle,
) {
	m.handler.refreshRecursiveGridForMonitorMove(targetBounds)
}

// RefreshForThemeChange draws the region the user has zoomed to again, so it
// picks up the colors the overlay just re-resolved. The zoom history is
// untouched: only the palette changed, not the screen underneath it.
//
// A mode with no manager has no region to draw, and says so rather than
// reporting a redraw the overlay never received.
func (m *RecursiveGridMode) RefreshForThemeChange() bool {
	handler := m.handler

	if handler.recursiveGrid == nil || handler.recursiveGrid.Manager == nil {
		return false
	}

	handler.updateRecursiveGridOverlay()
	handler.refreshRecursiveGridVirtualPointer()

	return true
}

// RefreshForScreenChange remaps the zoom history onto the display as it now is,
// so the region the user had narrowed to survives a display change instead of
// starting over: recursive grid is a sequence of narrowing choices, and
// throwing the region away throws their progress away with it.
//
// Recursive grid switched off in configuration, or with no component to remap,
// leaves the overlay still sized for the display that is gone, so the caller
// resizes it.
func (m *RecursiveGridMode) RefreshForScreenChange(_ context.Context) bool {
	handler := m.handler

	if handler.config == nil || !handler.config.RecursiveGrid.Enabled ||
		handler.recursiveGrid == nil {
		return false
	}

	handler.refreshRecursiveGridForScreenChange()

	handler.logger.Debug("Recursive-grid overlay resized and regenerated for new screen bounds")

	return true
}

// Exit tears recursive-grid mode down.
func (m *RecursiveGridMode) Exit() {
	m.handler.cleanupRecursiveGridMode()
}

// SelectionPoint reports the region center recursive-grid mode has selected.
func (m *RecursiveGridMode) SelectionPoint() (image.Point, bool) {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Context == nil {
		return image.Point{}, false
	}

	return m.handler.recursiveGrid.Context.SelectionPoint()
}

// ClearSelectionPoint forgets the selected region center and takes the virtual
// pointer that stood on it off the screen.
func (m *RecursiveGridMode) ClearSelectionPoint() bool {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Context == nil {
		return false
	}

	m.handler.recursiveGrid.Context.ClearSelectionPoint()
	m.handler.refreshRecursiveGridVirtualPointer()

	return true
}

// SelectionAnchor anchors to the selected region center, which is where the
// user is looking whenever the real cursor is not following the selection
// itself.
func (m *RecursiveGridMode) SelectionAnchor() (image.Point, bool) {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Context == nil ||
		m.handler.recursiveGrid.Context.CursorFollowSelection() {
		return image.Point{}, false
	}

	return m.handler.recursiveGrid.Context.SelectionPoint()
}

// MoveCell slides the highlighted region at the current depth, crossing into a
// neighboring parent when it runs off the edge of its own. A move that would
// leave the screen is refused by the manager.
func (m *RecursiveGridMode) MoveCell(dir domain.Direction, count int) {
	handler := m.handler

	if handler.recursiveGrid == nil || handler.recursiveGrid.Manager == nil {
		return
	}

	center, moved := handler.recursiveGrid.Manager.MoveDirection(dir, count)
	if !moved {
		return
	}

	// The update callback set the selection point and redrew the overlay;
	// only cursor tracking is left.
	absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, handler.screenBounds)

	if handler.recursiveGrid.Context != nil &&
		!handler.recursiveGrid.Context.CursorFollowSelection() {
		handler.refreshRecursiveGridVirtualPointer()

		return
	}

	err := handler.actionService.MoveCursorToPoint(handler.ctx, absoluteCenter)
	if err != nil {
		handler.logger.Error(
			"Failed to move cursor after recursive-grid cell move",
			zap.Error(err),
		)
	}
}

// CursorFollowSelection reports whether the real cursor rides along with the
// zoomed region's center.
func (m *RecursiveGridMode) CursorFollowSelection() (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	return modeContext.CursorFollowSelection(), true
}

// ApplyCursorFollowSelection sets or toggles the preference and then settles
// what the change owes: the virtual pointer stands in for the real cursor only
// while the cursor is not following, and turning following on puts the cursor
// on the region center that is already selected.
func (m *RecursiveGridMode) ApplyCursorFollowSelection(desired *bool) (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	enabled := applyCursorFollow(modeContext, desired)

	m.handler.refreshRecursiveGridVirtualPointer()
	m.handler.moveCursorToSelection(enabled, m.handler.recursiveGrid.Context.SelectionPoint)

	return enabled, true
}

// ExitSteps reports the --on-exit steps this recursive-grid session was
// activated with.
func (m *RecursiveGridMode) ExitSteps() []string {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Context == nil {
		return nil
	}

	return m.handler.recursiveGrid.Context.OnExit()
}

// ResetInput climbs all the way back out to the full screen, discarding the
// zoom history.
func (m *RecursiveGridMode) ResetInput() {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Manager == nil {
		return
	}

	m.handler.recursiveGrid.Manager.Reset()

	m.settleAtCurrentCenter("Failed to move cursor after recursive-grid reset")
}

// Backspace climbs one level back out of the zoom. A backtrack that had nowhere
// to go leaves the region, and the cursor, where they are.
func (m *RecursiveGridMode) Backspace() {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Manager == nil ||
		!m.handler.recursiveGrid.Manager.Backtrack() {
		return
	}

	m.settleAtCurrentCenter("Failed to move cursor after recursive-grid backspace")
}

// HasAppHotkeyOverrides reports whether [recursive_grid.apps] binds any per-app
// hotkey.
func (m *RecursiveGridMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.RecursiveGrid.HasAppHotkeyOverrides()
}

// cursorFollowContext is the recursive-grid session's preference carrier, or
// false when there is no session.
func (m *RecursiveGridMode) cursorFollowContext() (cursorFollowContext, bool) {
	if m.handler.recursiveGrid == nil || m.handler.recursiveGrid.Context == nil {
		return nil, false
	}

	return m.handler.recursiveGrid.Context, true
}

// settleAtCurrentCenter re-selects the center of the region the manager now
// holds, redraws it, and brings whichever pointer is standing in for the
// selection — the virtual one, or the real cursor when it follows — onto it.
func (m *RecursiveGridMode) settleAtCurrentCenter(moveFailureMessage string) {
	handler := m.handler

	center := handler.recursiveGrid.Manager.CurrentCenter()

	absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, handler.screenBounds)
	if handler.recursiveGrid.Context != nil {
		handler.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
	}

	handler.updateRecursiveGridOverlay()

	if handler.recursiveGrid.Context != nil &&
		!handler.recursiveGrid.Context.CursorFollowSelection() {
		handler.refreshRecursiveGridVirtualPointer()

		return
	}

	err := handler.actionService.MoveCursorToPoint(handler.ctx, absoluteCenter)
	if err != nil {
		handler.logger.Error(moveFailureMessage, zap.Error(err))
	}
}
