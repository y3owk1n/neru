package hints

import (
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
)

// Context holds the state and context for hint mode operations.
type Context struct {
	Manager       *domainHint.Manager
	Router        *domainHint.Router
	Hints         *domainHint.Collection
	InActionMode  bool
	PendingAction *string
}

// SetManager sets the domain hint manager.
func (c *Context) SetManager(manager *domainHint.Manager) {
	c.Manager = manager
}

// GetManager returns the domain hint manager.
func (c *Context) GetManager() *domainHint.Manager {
	return c.Manager
}

// SetRouter sets the domain hint router.
func (c *Context) SetRouter(router *domainHint.Router) {
	c.Router = router
}

// GetRouter returns the domain hint router.
func (c *Context) GetRouter() *domainHint.Router {
	return c.Router
}

// SetHints sets the current hint collection.
func (c *Context) SetHints(hints *domainHint.Collection) {
	c.Hints = hints
	if c.Manager != nil {
		c.Manager.SetHints(hints)
	}
}

// GetHints returns the current hint collection.
func (c *Context) GetHints() *domainHint.Collection {
	return c.Hints
}

// SetInActionMode sets whether hint mode is in action mode.
func (c *Context) SetInActionMode(inActionMode bool) {
	c.InActionMode = inActionMode
}

// GetInActionMode returns whether hint mode is in action mode.
func (c *Context) GetInActionMode() bool {
	return c.InActionMode
}

// SetPendingAction sets the action to execute when a hint is selected.
func (c *Context) SetPendingAction(action *string) {
	c.PendingAction = action
}

// GetPendingAction returns the pending action to execute.
func (c *Context) GetPendingAction() *string {
	return c.PendingAction
}

// Reset resets the hints context to its initial state.
func (c *Context) Reset() {
	if c.Manager != nil {
		c.Manager.Reset()
	}
	c.InActionMode = false
	c.PendingAction = nil
}
