package grid

import (
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// baseContext provides common functionality for mode component contexts.
// It contains shared state fields used across different mode contexts.
type baseContext struct {
	inActionMode  bool
	pendingAction *string
}

// SetInActionMode sets whether the mode is in action mode.
func (c *baseContext) SetInActionMode(inActionMode bool) {
	c.inActionMode = inActionMode
}

// InActionMode returns whether the mode is in action mode.
func (c *baseContext) InActionMode() bool {
	return c.inActionMode
}

// SetPendingAction sets the action to execute when mode selection is complete.
func (c *baseContext) SetPendingAction(action *string) {
	c.pendingAction = action
}

// PendingAction returns the pending action to execute.
func (c *baseContext) PendingAction() *string {
	return c.pendingAction
}

// Reset resets the base context to its initial state.
func (c *baseContext) Reset() {
	c.inActionMode = false
	c.pendingAction = nil
}

// Context holds the state and context for grid mode operations.
type Context struct {
	baseContext

	gridInstance **domainGrid.Grid
	gridOverlay  **Overlay
}

// SetGridInstance sets the grid instance.
func (c *Context) SetGridInstance(gridInstance **domainGrid.Grid) {
	c.gridInstance = gridInstance
}

// SetGridInstanceValue sets the value of the grid instance pointer.
func (c *Context) SetGridInstanceValue(gridInstance *domainGrid.Grid) {
	*c.gridInstance = gridInstance
}

// GridInstance returns the grid instance.
func (c *Context) GridInstance() **domainGrid.Grid {
	return c.gridInstance
}

// SetGridOverlay sets the grid overlay.
func (c *Context) SetGridOverlay(gridOverlay **Overlay) {
	c.gridOverlay = gridOverlay
}

// SetGridOverlayValue sets the value of the grid overlay pointer.
func (c *Context) SetGridOverlayValue(gridOverlay *Overlay) {
	*c.gridOverlay = gridOverlay
}

// GridOverlay returns the grid overlay.
func (c *Context) GridOverlay() **Overlay {
	return c.gridOverlay
}

// Reset resets the grid context to its initial state.
func (c *Context) Reset() {
	c.gridInstance = nil
	c.gridOverlay = nil
	c.baseContext.Reset()
}
