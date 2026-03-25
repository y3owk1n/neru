package scroll

import (
	"sync"
)

// Context holds the state and context for scroll mode operations.
type Context struct {
	mu sync.RWMutex

	// isActive indicates whether scroll mode is currently active
	isActive bool
}

// SetIsActive sets whether scroll mode is currently active.
func (c *Context) SetIsActive(active bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isActive = active
}

// IsActive returns whether scroll mode is currently active.
func (c *Context) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isActive
}

// Reset resets the scroll context to its initial state.
func (c *Context) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isActive = false
}
