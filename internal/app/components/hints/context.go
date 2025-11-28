package hints

import (
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
)

// Context holds the state and context for hint mode operations.
type Context struct {
	manager       *domainHint.Manager
	router        *domainHint.Router
	hints         *domainHint.Collection
	inActionMode  bool
	pendingAction *string
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

// SetInActionMode sets whether hint mode is in action mode.
func (c *Context) SetInActionMode(inActionMode bool) {
	c.inActionMode = inActionMode
}

// InActionMode returns whether hint mode is in action mode.
func (c *Context) InActionMode() bool {
	return c.inActionMode
}

// SetPendingAction sets the action to execute when a hint is selected.
func (c *Context) SetPendingAction(action *string) {
	c.pendingAction = action
}

// PendingAction returns the pending action to execute.
func (c *Context) PendingAction() *string {
	return c.pendingAction
}

// Reset resets the hints context to its initial state.
func (c *Context) Reset() {
	if c.manager != nil {
		c.manager.Reset()
	}

	c.inActionMode = false
	c.pendingAction = nil
}
