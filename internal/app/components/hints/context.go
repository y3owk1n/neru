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
	filterRoles           []string
	filterTextContains    []string
	startWithSearch       bool
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
	c.filterRoles = nil
	c.filterTextContains = nil
	c.startWithSearch = false
}

// SetFilterRoles sets the filter roles for hint mode.
func (c *baseContext) SetFilterRoles(roles []string) {
	c.filterRoles = roles
}

// FilterRoles returns the filter roles.
func (c *baseContext) FilterRoles() []string {
	return c.filterRoles
}

// SetFilterTextContains sets the filter text for hint mode.
func (c *baseContext) SetFilterTextContains(texts []string) {
	c.filterTextContains = texts
}

// FilterTextContains returns the filter text.
func (c *baseContext) FilterTextContains() []string {
	return c.filterTextContains
}

// SetStartWithSearch sets whether the mode was initially activated with search.
func (c *baseContext) SetStartWithSearch(search bool) {
	c.startWithSearch = search
}

// StartWithSearch returns whether the mode was initially activated with search.
func (c *baseContext) StartWithSearch() bool {
	return c.startWithSearch
}

// Context holds the state and context for hint mode operations.
type Context struct {
	baseContext

	manager     *domainHint.Manager
	router      *domainHint.Router
	hints       *domainHint.Collection
	sourceHints *domainHint.Collection

	searchQuery  string
	searchActive bool
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
func (c *Context) SetHints(hints *domainHint.Collection) error {
	if c.manager != nil {
		err := c.manager.SetHints(hints)
		if err != nil {
			return err
		}
	}

	c.hints = hints
	c.sourceHints = hints

	return nil
}

// SetVisibleHints sets the currently selectable hint collection without
// replacing the original source collection used by search cancellation.
func (c *Context) SetVisibleHints(hints *domainHint.Collection) error {
	if c.manager != nil {
		err := c.manager.SetHints(hints)
		if err != nil {
			return err
		}
	}

	c.hints = hints

	return nil
}

// Hints returns the current hint collection.
func (c *Context) Hints() *domainHint.Collection {
	return c.hints
}

// SourceHints returns the unfiltered hint collection from mode activation.
func (c *Context) SourceHints() *domainHint.Collection {
	return c.sourceHints
}

// SetSearchQuery sets the current hint text search query.
func (c *Context) SetSearchQuery(query string) {
	c.searchQuery = query
}

// SearchQuery returns the current hint text search query.
func (c *Context) SearchQuery() string {
	return c.searchQuery
}

// SetSearchActive sets whether hint text search is active.
func (c *Context) SetSearchActive(active bool) {
	c.searchActive = active
}

// SearchActive returns whether hint text search is active.
func (c *Context) SearchActive() bool {
	return c.searchActive
}

// Reset resets the hints context to its initial state.
func (c *Context) Reset() error {
	var err error
	if c.manager != nil {
		err = c.manager.Clear()
	}

	c.hints = nil
	c.sourceHints = nil
	c.searchQuery = ""
	c.searchActive = false
	c.baseContext.Reset()

	return err
}
