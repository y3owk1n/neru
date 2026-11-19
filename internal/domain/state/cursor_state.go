package state

import "sync"

// CursorState tracks per-session cursor flags, such as whether a click was
// performed and so needs a settle delay before the cursor moves again.
type CursorState struct {
	mu sync.RWMutex

	actionPerformed bool
}

// NewCursorState creates a new CursorState.
func NewCursorState() *CursorState {
	return &CursorState{}
}

// Reset clears the session flags.
func (c *CursorState) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.actionPerformed = false
}

// MarkActionPerformed records that a click/mouse action was just executed.
// handleCursorRestoration uses this to insert a settling delay before moving
// the cursor, giving the target application time to finish processing the click.
func (c *CursorState) MarkActionPerformed() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.actionPerformed = true
}

// WasActionPerformed returns whether a click action was performed in this
// mode session. The flag is cleared by Reset.
func (c *CursorState) WasActionPerformed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.actionPerformed
}
