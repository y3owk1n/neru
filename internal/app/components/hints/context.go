package hints

import (
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
)

// baseContext provides common functionality for mode component contexts.
// It contains shared state fields used across different mode contexts.
type baseContext struct {
	pendingAction         *string
	repeat                bool
	cursorFollowSelection bool
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

// SetCursorFollowSelection stores the session cursor-follow-selection preference.
func (c *baseContext) SetCursorFollowSelection(cursorFollowSelection bool) {
	c.cursorFollowSelection = cursorFollowSelection
}

// CursorFollowSelection returns the current session cursor-follow-selection preference.
func (c *baseContext) CursorFollowSelection() bool {
	return c.cursorFollowSelection
}

// ToggleCursorFollowSelection flips the session cursor-follow-selection preference.
func (c *baseContext) ToggleCursorFollowSelection() bool {
	c.cursorFollowSelection = !c.cursorFollowSelection

	return c.cursorFollowSelection
}

// Reset resets the base context to its initial state.
func (c *baseContext) Reset() {
	c.pendingAction = nil
	c.repeat = false
	c.cursorFollowSelection = false
}

// Context holds the state and context for hint mode operations.
type Context struct {
	baseContext

	manager *domainHint.Manager
	router  *domainHint.Router
	hints   *domainHint.Collection
}

// SetManager sets the domain hint manager.
func (c *Context) SetManager(manager *domainHint.Manager) {
	c.manager = manager
}

// Manager returns the domain hint manager.
func (c *Context) Manager() *domainHint.Manager {
	return c.manager
}

// SetRouter sets the domain hint router.
func (c *Context) SetRouter(router *domainHint.Router) {
	c.router = router
}

// Router returns the domain hint router.
func (c *Context) Router() *domainHint.Router {
	return c.router
}

// SetHints sets the current hint collection.
func (c *Context) SetHints(hints *domainHint.Collection) {
	c.hints = hints
	if c.manager != nil {
		c.manager.SetHints(hints)
	}
}

// Hints returns the current hint collection.
func (c *Context) Hints() *domainHint.Collection {
	return c.hints
}

// Reset resets the hints context to its initial state.
func (c *Context) Reset() {
	if c.manager != nil {
		c.manager.Reset()
	}

	c.baseContext.Reset()
}
