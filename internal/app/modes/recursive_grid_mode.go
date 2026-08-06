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
	_ Mode             = (*RecursiveGridMode)(nil)
	_ selectionTracker = (*RecursiveGridMode)(nil)
	_ cellNavigator    = (*RecursiveGridMode)(nil)
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
