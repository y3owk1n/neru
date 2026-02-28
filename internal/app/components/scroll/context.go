package scroll

import (
	"sync"
	"time"
)

// Context holds the state and context for scroll mode operations.
type Context struct {
	mu sync.RWMutex

	// lastKey tracks the last key pressed during scroll operations
	// This is used for multi-key operations like "gg" for top
	lastKey string

	// lastKeyTime tracks when the last key was pressed for sequence timeout
	lastKeyTime int64

	// isActive indicates whether scroll mode is currently active
	isActive bool
}

// SetLastKey sets the last key pressed during scroll operations.
func (c *Context) SetLastKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastKey = key
	c.lastKeyTime = time.Now().UnixNano()
}

// LastKey returns the last key pressed during scroll operations.
func (c *Context) LastKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastKey
}

// LastKeyTime returns the timestamp when the last key was pressed.
func (c *Context) LastKeyTime() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastKeyTime
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

	c.lastKey = ""
	c.lastKeyTime = 0
	c.isActive = false
}
