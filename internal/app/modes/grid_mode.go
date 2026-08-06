package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension grid mode opts into (extensions.go).
var (
	_ Mode                   = (*GridMode)(nil)
	_ selectionTracker       = (*GridMode)(nil)
	_ cellNavigator          = (*GridMode)(nil)
	_ cursorFollowSelector   = (*GridMode)(nil)
	_ exitStepReporter       = (*GridMode)(nil)
	_ inputEditor            = (*GridMode)(nil)
	_ hotkeyOverrideReporter = (*GridMode)(nil)
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

// RefreshForMonitorMove rebuilds the grid for the display the cursor landed on
// and hands it over as a Frame. The old cells describe a screen the user is no
// longer looking at, so the input and the selection go with them.
func (m *GridMode) RefreshForMonitorMove(_ context.Context, targetBounds image.Rectangle) {
	m.handler.refreshGridForMonitorMove(targetBounds)
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

// CursorFollowSelection reports whether the real cursor rides along with the
// selected cell.
func (m *GridMode) CursorFollowSelection() (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	return modeContext.CursorFollowSelection(), true
}

// ApplyCursorFollowSelection sets or toggles the preference and then settles
// what the change owes: the virtual pointer stands in for the real cursor only
// while the cursor is not following, and turning following on puts the cursor
// on the cell that is already selected.
func (m *GridMode) ApplyCursorFollowSelection(desired *bool) (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	enabled := applyCursorFollow(modeContext, desired)

	m.handler.refreshGridVirtualPointer()
	m.handler.moveCursorToSelection(enabled, m.handler.grid.Context.SelectionPoint)

	return enabled, true
}

// ExitSteps reports the --on-exit steps this grid session was activated with.
func (m *GridMode) ExitSteps() []string {
	if m.handler.grid == nil || m.handler.grid.Context == nil {
		return nil
	}

	return m.handler.grid.Context.OnExit()
}

// ResetInput clears the typed cell label and redraws the full grid. The
// selection goes with the input: nothing is typed, so no cell is chosen.
func (m *GridMode) ResetInput() {
	handler := m.handler

	if handler.grid == nil || handler.grid.Manager == nil {
		return
	}

	handler.grid.Manager.Reset()

	// Clear stale selection — input was reset so no cell is selected. The
	// session builds its Context and Manager together, so the guard above
	// answers for both.
	handler.grid.Context.ClearSelectionPoint()

	gridInstancePtr := handler.grid.Context.GridInstance()
	if gridInstancePtr == nil || *gridInstancePtr == nil {
		return
	}

	handler.redrawFrame(
		ports.GridFrame{
			Grid:  *gridInstancePtr,
			Input: handler.grid.Manager.CurrentInput(),
		},
		"redraw grid after reset",
	)

	handler.refreshGridVirtualPointer()
}

// Backspace takes back the last character of the cell label being typed. The
// manager's update callback redraws what is left.
func (m *GridMode) Backspace() {
	if m.handler.grid == nil || m.handler.grid.Manager == nil {
		return
	}

	m.handler.grid.Manager.HandleBackspace()
}

// HasAppHotkeyOverrides reports whether [grid.apps] binds any per-app hotkey.
func (m *GridMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Grid.HasAppHotkeyOverrides()
}

// cursorFollowContext is the grid session's preference carrier, or false when
// there is no session.
func (m *GridMode) cursorFollowContext() (cursorFollowContext, bool) {
	if m.handler.grid == nil || m.handler.grid.Context == nil {
		return nil, false
	}

	return m.handler.grid.Context, true
}
