package quadgrid

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// Manager handles quad-grid navigation state and input processing.
type Manager struct {
	domain.BaseManager

	grid       *QuadGrid
	keys       string            // Key mapping (e.g., "uijk")
	onUpdate   func()            // Callback for overlay updates
	onComplete func(image.Point) // Callback when selection is complete
	resetKey   string
	exitKeys   []string
}

// NewManager creates a quad-grid manager with the specified configuration.
func NewManager(
	screenBounds image.Rectangle,
	keys string,
	resetKey string,
	exitKeys []string,
	onUpdate func(),
	onComplete func(image.Point),
	logger *zap.Logger,
) *Manager {
	// Use default keys if not provided
	if strings.TrimSpace(keys) == "" {
		keys = DefaultKeys
	}

	// Ensure we have exactly 4 keys
	if utf8.RuneCountInString(keys) != 4 { //nolint:mnd
		logger.Warn("Invalid key mapping length, using default",
			zap.String("provided", keys),
			zap.Int("length", utf8.RuneCountInString(keys)))
		keys = DefaultKeys
	}

	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		// Default: 25px min, 10 max depth
		grid:       NewQuadGrid(screenBounds, 25, 10), //nolint:mnd
		keys:       strings.ToLower(keys),
		onUpdate:   onUpdate,
		onComplete: onComplete,
		resetKey:   resetKey,
		exitKeys:   exitKeys,
	}
}

// NewManagerWithConfig creates a manager with custom minSize and maxDepth.
func NewManagerWithConfig(
	screenBounds image.Rectangle,
	keys string,
	resetKey string,
	exitKeys []string,
	minSize, maxDepth int,
	onUpdate func(),
	onComplete func(image.Point),
	logger *zap.Logger,
) *Manager {
	manager := NewManager(screenBounds, keys, resetKey, exitKeys, onUpdate, onComplete, logger)
	// Replace the grid with custom configuration
	manager.grid = NewQuadGrid(screenBounds, minSize, maxDepth)

	return manager
}

// HandleInput processes a key press and updates the grid state.
// Returns the new cursor position (if applicable), whether the selection is complete,
// and whether the mode should exit.
func (m *Manager) HandleInput(key string) (image.Point, bool, bool) {
	// Normalize key to lowercase for comparison
	key = strings.ToLower(key)

	// Check exit keys first
	for _, exitKey := range m.exitKeys {
		if config.IsExitKey(key, []string{exitKey}) {
			m.Logger.Debug("Exit key pressed in quad-grid mode",
				zap.String("key", key))

			return image.Point{}, false, true
		}
	}

	// Handle reset key
	if config.IsResetKey(key, m.resetKey) {
		m.Logger.Debug("Reset key pressed in quad-grid mode",
			zap.String("key", key))
		m.Reset()

		if m.onUpdate != nil {
			m.onUpdate()
		}

		// Return initial center to move cursor back
		return m.grid.CurrentCenter(), false, false
	}

	// Handle backspace/delete for backtracking
	if key == "\x7f" || key == "delete" || key == "backspace" {
		if m.grid.Backtrack() {
			m.Logger.Debug("Backtracked in quad-grid mode",
				zap.Int("new_depth", m.grid.CurrentDepth()))

			if m.onUpdate != nil {
				m.onUpdate()
			}

			// Return new center (of parent quadrant) to move cursor
			return m.grid.CurrentCenter(), false, false
		}

		return image.Point{}, false, false
	}

	// Map key to quadrant
	quadrant := m.keyToQuadrant(key)
	if quadrant < 0 {
		// Key not mapped to any quadrant
		m.Logger.Debug("Unmapped key pressed in quad-grid mode",
			zap.String("key", key))

		return image.Point{}, false, false
	}

	// Select the quadrant
	center, isComplete := m.grid.SelectQuadrant(quadrant)

	m.Logger.Debug("Quadrant selected",
		zap.String("key", key),
		zap.Int("quadrant", int(quadrant)),
		zap.Int("depth", m.grid.CurrentDepth()),
		zap.Bool("complete", isComplete),
		zap.Int("center_x", center.X),
		zap.Int("center_y", center.Y))

	// Trigger update for visual feedback
	if m.onUpdate != nil {
		m.onUpdate()
	}

	// If complete, call the completion callback
	if isComplete && m.onComplete != nil {
		m.onComplete(center)
	}

	return center, isComplete, false
}

// Reset clears the manager state and restores initial grid state.
func (m *Manager) Reset() {
	m.SetCurrentInput("")
	m.grid.Reset()
}

// CurrentGrid returns the underlying QuadGrid instance.
func (m *Manager) CurrentGrid() *QuadGrid {
	return m.grid
}

// CurrentBounds returns the current active bounds.
func (m *Manager) CurrentBounds() image.Rectangle {
	return m.grid.CurrentBounds()
}

// CurrentDepth returns the current recursion depth.
func (m *Manager) CurrentDepth() int {
	return m.grid.CurrentDepth()
}

// IsComplete returns true if the minimum size has been reached.
func (m *Manager) IsComplete() bool {
	return m.grid.IsComplete()
}

// CanDivide returns true if the current bounds can be divided further.
func (m *Manager) CanDivide() bool {
	return m.grid.CanDivide()
}

// CurrentCenter returns the center point of the current bounds.
func (m *Manager) CurrentCenter() image.Point {
	return m.grid.CurrentCenter()
}

// QuadrantCenter returns the center point for a specific quadrant.
func (m *Manager) QuadrantCenter(q Quadrant) image.Point {
	return m.grid.QuadrantCenter(q)
}

// QuadrantBounds returns the bounds for a specific quadrant.
func (m *Manager) QuadrantBounds(q Quadrant) image.Rectangle {
	return m.grid.QuadrantBounds(q)
}

// Keys returns the current key mapping.
func (m *Manager) Keys() string {
	return m.keys
}

// UpdateKeys updates the key mapping.
func (m *Manager) UpdateKeys(keys string) {
	if utf8.RuneCountInString(keys) == 4 { //nolint:mnd
		m.keys = strings.ToLower(keys)
	}
}

// Backtrack returns to the previous bounds.
// Returns true if backtracking was successful.
func (m *Manager) Backtrack() bool {
	return m.grid.Backtrack()
}

// HasHistory returns true if there's backtrack history available.
func (m *Manager) HasHistory() bool {
	return m.grid.HasHistory()
}

// keyToQuadrant maps an input key to a quadrant index.
// Returns -1 if the key is not mapped.
func (m *Manager) keyToQuadrant(key string) Quadrant {
	idx := 0
	for _, k := range m.keys {
		if string(k) == key {
			return Quadrant(idx)
		}

		idx++
	}

	return -1
}
