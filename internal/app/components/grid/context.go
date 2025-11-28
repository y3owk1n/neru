package grid

import (
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// Context holds the state and context for grid mode operations.
type Context struct {
	gridInstance  **domainGrid.Grid
	gridOverlay   **Overlay
	inActionMode  bool
	pendingAction *string
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

// SetInActionMode sets whether grid mode is in action mode.
func (c *Context) SetInActionMode(inActionMode bool) {
	c.inActionMode = inActionMode
}

// InActionMode returns whether grid mode is in action mode.
func (c *Context) InActionMode() bool {
	return c.inActionMode
}

// SetPendingAction sets the action to execute when grid selection is complete.
func (c *Context) SetPendingAction(action *string) {
	c.pendingAction = action
}

// PendingAction returns the pending action to execute.
func (c *Context) PendingAction() *string {
	return c.pendingAction
}

// Reset resets the grid context to its initial state.
func (c *Context) Reset() {
	c.gridInstance = nil
	c.gridOverlay = nil
	c.inActionMode = false
	c.pendingAction = nil
}
