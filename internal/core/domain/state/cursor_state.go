package state

import (
	"image"
	"sync"
)

// CursorState manages cursor position tracking and restoration functionality.
//
// This state is used to remember the cursor position before activating
// hint/grid modes and optionally restore it after the action is complete.
type CursorState struct {
	mu sync.RWMutex

	// Capture state
	initialPos          image.Point
	initialScreenBounds image.Rectangle
	captured            bool

	// Behavior flags
	restoreEnabled  bool
	skipRestoreOnce bool
}

// NewCursorState creates a new CursorState with the specified restore behavior.
func NewCursorState(restoreEnabled bool) *CursorState {
	return &CursorState{
		restoreEnabled: restoreEnabled,
	}
}

// Capture stores the current cursor position and screen bounds.
func (c *CursorState) Capture(pos image.Point, bounds image.Rectangle) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.initialPos = pos
	c.initialScreenBounds = bounds
	c.captured = true
}

// IsCaptured returns whether cursor position has been captured.
func (c *CursorState) IsCaptured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.captured
}

// Reset clears the captured cursor state.
func (c *CursorState) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.captured = false
	c.skipRestoreOnce = false
}

// InitialPosition returns the captured cursor position.
func (c *CursorState) InitialPosition() image.Point {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.initialPos
}

// InitialScreenBounds returns the captured screen bounds.
func (c *CursorState) InitialScreenBounds() image.Rectangle {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.initialScreenBounds
}

// ShouldRestore returns whether the cursor should be restored.
// It considers both the restore enabled flag and the skip restore flag.
func (c *CursorState) ShouldRestore() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.restoreEnabled && c.captured && !c.skipRestoreOnce
}

// SkipNextRestore sets a flag to skip the next cursor restoration.
// This is useful for operations that want to leave the cursor at its new position.
func (c *CursorState) SkipNextRestore() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.skipRestoreOnce = true
}

// SetRestoreEnabled enables or disables cursor restoration.
func (c *CursorState) SetRestoreEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.restoreEnabled = enabled
}

// IsRestoreEnabled returns whether cursor restoration is enabled.
func (c *CursorState) IsRestoreEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.restoreEnabled
}
