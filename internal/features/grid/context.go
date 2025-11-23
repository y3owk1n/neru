package grid

import (
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// Context holds the state and context for grid mode operations.
type Context struct {
	GridInstance  **domainGrid.Grid
	GridOverlay   **Overlay
	InActionMode  bool
	PendingAction *string
}

// SetGridInstance sets the grid instance.
func (c *Context) SetGridInstance(gridInstance **domainGrid.Grid) {
	c.GridInstance = gridInstance
}

// SetGridInstanceValue sets the value of the grid instance pointer.
func (c *Context) SetGridInstanceValue(gridInstance *domainGrid.Grid) {
	*c.GridInstance = gridInstance
}

// GetGridInstance returns the grid instance.
func (c *Context) GetGridInstance() **domainGrid.Grid {
	return c.GridInstance
}

// SetGridOverlay sets the grid overlay.
func (c *Context) SetGridOverlay(gridOverlay **Overlay) {
	c.GridOverlay = gridOverlay
}

// SetGridOverlayValue sets the value of the grid overlay pointer.
func (c *Context) SetGridOverlayValue(gridOverlay *Overlay) {
	*c.GridOverlay = gridOverlay
}

// GetGridOverlay returns the grid overlay.
func (c *Context) GetGridOverlay() **Overlay {
	return c.GridOverlay
}

// SetInActionMode sets whether grid mode is in action mode.
func (c *Context) SetInActionMode(inActionMode bool) {
	c.InActionMode = inActionMode
}

// GetInActionMode returns whether grid mode is in action mode.
func (c *Context) GetInActionMode() bool {
	return c.InActionMode
}

// SetPendingAction sets the action to execute when grid selection is complete.
func (c *Context) SetPendingAction(action *string) {
	c.PendingAction = action
}

// GetPendingAction returns the pending action to execute.
func (c *Context) GetPendingAction() *string {
	return c.PendingAction
}

// Reset resets the grid context to its initial state.
func (c *Context) Reset() {
	c.GridInstance = nil
	c.GridOverlay = nil
	c.InActionMode = false
	c.PendingAction = nil
}
