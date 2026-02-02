// Package quadgrid provides the quad-grid component.
package quadgrid

import (
	"github.com/y3owk1n/neru/internal/core/domain/quadgrid"
)

// baseContext provides common functionality for mode component contexts.
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

// Context holds the state and context for quad-grid mode operations.
type Context struct {
	baseContext
}

// Reset resets the quad-grid context to its initial state.
func (c *Context) Reset() {
	c.baseContext.Reset()
}

// Component holds the components for quad-grid mode.
type Component struct {
	Manager *quadgrid.Manager
	Overlay *Overlay
	Context *Context
	Style   Style
}
