package scroll

import "time"

// Context holds the state and context for scroll mode operations.
type Context struct {
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
	c.lastKey = key
	c.lastKeyTime = time.Now().UnixNano()
}

// LastKey returns the last key pressed during scroll operations.
func (c *Context) LastKey() string {
	return c.lastKey
}

// LastKeyTime returns the timestamp when the last key was pressed.
func (c *Context) LastKeyTime() int64 {
	return c.lastKeyTime
}

// SetIsActive sets whether scroll mode is currently active.
func (c *Context) SetIsActive(active bool) {
	c.isActive = active
}

// IsActive returns whether scroll mode is currently active.
func (c *Context) IsActive() bool {
	return c.isActive
}

// Reset resets the scroll context to its initial state.
func (c *Context) Reset() {
	c.lastKey = ""
	c.lastKeyTime = 0
	c.isActive = false
}
