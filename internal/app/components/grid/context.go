package grid

import (
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// baseContext provides common functionality for mode component contexts.
// It contains shared state fields used across different mode contexts.
type baseContext struct {
	pendingAction *string
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
	c.pendingAction = nil
}

// Context holds the state and context for grid mode operations.
type Context struct {
	baseContext

	gridInstance **domainGrid.Grid
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

// Reset resets the grid context to its initial state.
func (c *Context) Reset() {
	c.gridInstance = nil
	c.baseContext.Reset()
}
