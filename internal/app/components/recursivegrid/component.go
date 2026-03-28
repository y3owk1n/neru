// Package recursivegrid provides the recursivegrid component.
package recursivegrid

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
)

// baseContext provides common functionality for mode component contexts.
type baseContext struct {
	pendingAction         *string
	repeat                bool
	cursorFollowSelection bool
	selectedPoint         image.Point
	hasSelection          bool
}

// SetPendingAction sets the action to execute when mode selection is complete.
func (c *baseContext) SetPendingAction(action *string) {
	c.pendingAction = action
}

// PendingAction returns the pending action to execute.
func (c *baseContext) PendingAction() *string {
	return c.pendingAction
}

// SetRepeat sets whether the mode should re-activate after performing the action.
func (c *baseContext) SetRepeat(repeat bool) {
	c.repeat = repeat
}

// Repeat returns whether the mode should re-activate after performing the action.
func (c *baseContext) Repeat() bool {
	return c.repeat
}

// SetCursorFollowSelection sets whether live selection updates should move the real cursor.
func (c *baseContext) SetCursorFollowSelection(cursorFollowSelection bool) {
	c.cursorFollowSelection = cursorFollowSelection
}

// CursorFollowSelection returns whether live selection updates should move the real cursor.
func (c *baseContext) CursorFollowSelection() bool {
	return c.cursorFollowSelection
}

// ToggleCursorFollowSelection flips the live cursor tracking state and returns the new value.
func (c *baseContext) ToggleCursorFollowSelection() bool {
	c.cursorFollowSelection = !c.cursorFollowSelection

	return c.cursorFollowSelection
}

// SetSelectionPoint stores the active selection point for the mode.
func (c *baseContext) SetSelectionPoint(point image.Point) {
	c.selectedPoint = point
	c.hasSelection = true
}

// ClearSelectionPoint removes the active selection point for the mode.
func (c *baseContext) ClearSelectionPoint() {
	c.selectedPoint = image.Point{}
	c.hasSelection = false
}

// SelectionPoint returns the active selection point for the mode, if any.
func (c *baseContext) SelectionPoint() (image.Point, bool) {
	return c.selectedPoint, c.hasSelection
}

// Reset resets the base context to its initial state.
func (c *baseContext) Reset() {
	c.pendingAction = nil
	c.repeat = false
	c.cursorFollowSelection = false
	c.ClearSelectionPoint()
}

// Context holds the state and context for recursive_grid mode operations.
type Context struct {
	baseContext
}

// Reset resets the recursive_grid context to its initial state.
func (c *Context) Reset() {
	c.baseContext.Reset()
}

// Component holds the components for recursive_grid mode.
type Component struct {
	Manager *recursivegrid.Manager
	Overlay *Overlay
	Context *Context
	Style   Style
}
