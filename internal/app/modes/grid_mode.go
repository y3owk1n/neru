package modes

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension grid mode opts into (extensions.go).
var (
	_ Mode             = (*GridMode)(nil)
	_ selectionTracker = (*GridMode)(nil)
	_ cellNavigator    = (*GridMode)(nil)
)

// GridMode implements the Mode interface for grid-based navigation.
type GridMode struct {
	baseMode
}

// NewGridMode creates a new grid mode implementation.
func NewGridMode(handler *handlerState) *GridMode {
	return &GridMode{
		baseMode: newBaseMode(handler, domain.ModeGrid, "GridMode"),
	}
}

// Activate enters grid mode with the flags the activation carries.
func (m *GridMode) Activate(activation modecmd.Activation) {
	m.handler.activateGridModeWithAction(activation)
}

// HandleKey processes a key press within grid mode.
func (m *GridMode) HandleKey(key string) {
	m.handler.handleGridModeKey(key)
}

// Exit tears grid mode down.
func (m *GridMode) Exit() {
	m.handler.cleanupGridMode()
}

// SelectionPoint reports the cell grid mode currently has selected.
func (m *GridMode) SelectionPoint() (image.Point, bool) {
	if m.handler.grid == nil || m.handler.grid.Context == nil {
		return image.Point{}, false
	}

	return m.handler.grid.Context.SelectionPoint()
}

// ClearSelectionPoint forgets the selected cell and takes the virtual pointer
// that stood on it off the screen.
func (m *GridMode) ClearSelectionPoint() bool {
	if m.handler.grid == nil || m.handler.grid.Context == nil {
		return false
	}

	m.handler.grid.Context.ClearSelectionPoint()
	m.handler.refreshGridVirtualPointer()

	return true
}

// SelectionAnchor anchors to the selected cell, which is where the user is
// looking whenever the real cursor is not following the selection itself.
func (m *GridMode) SelectionAnchor() (image.Point, bool) {
	if m.handler.grid == nil || m.handler.grid.Context == nil ||
		m.handler.grid.Context.CursorFollowSelection() {
		return image.Point{}, false
	}

	return m.handler.grid.Context.SelectionPoint()
}

// MoveCell moves an open subgrid to the neighboring cell.
//
// The subgrid callback already sets the selection point, redraws the subgrid
// over the new cell, and moves the cursor when cursor-follow is on, so there is
// nothing left to do here.
func (m *GridMode) MoveCell(dir domain.Direction, count int) {
	if m.handler.grid == nil || m.handler.grid.Manager == nil {
		return
	}

	m.handler.grid.Manager.MoveDirection(dir, count)
}
