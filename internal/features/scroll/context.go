package scroll

// Context holds the state and context for scroll mode operations.
type Context struct {
	// lastKey tracks the last key pressed during scroll operations
	// This is used for multi-key operations like "gg" for top
	lastKey string

	// isActive indicates whether scroll mode is currently active
	isActive bool
}

// SetLastKey sets the last key pressed during scroll operations.
func (c *Context) SetLastKey(key string) {
	c.lastKey = key
}

// LastKey returns the last key pressed during scroll operations.
func (c *Context) LastKey() string {
	return c.lastKey
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
	c.isActive = false
}
