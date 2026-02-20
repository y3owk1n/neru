package recursivegrid

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// Manager handles recursive-grid navigation state and input processing.
type Manager struct {
	domain.BaseManager

	grid       *RecursiveGrid
	keys       string            // Key mapping (e.g., "uijk")
	gridSize   int               // Grid size: 2 for 2x2, 3 for 3x3
	onUpdate   func()            // Callback for overlay updates
	onComplete func(image.Point) // Callback when selection is complete
	resetKey   string
	exitKeys   []string
}

// NewManager creates a recursive-grid manager with the specified configuration.
func NewManager(
	screenBounds image.Rectangle,
	keys string,
	resetKey string,
	exitKeys []string,
	onUpdate func(),
	onComplete func(image.Point),
	logger *zap.Logger,
) *Manager {
	return NewManagerWithConfig(
		screenBounds,
		keys,
		resetKey,
		exitKeys,
		25, //nolint:mnd
		10, //nolint:mnd
		GridSize2x2,
		onUpdate,
		onComplete,
		logger,
	)
}

// NewManagerWithConfig creates a manager with custom minSize, maxDepth, and gridSize.
func NewManagerWithConfig(
	screenBounds image.Rectangle,
	keys string,
	resetKey string,
	exitKeys []string,
	minSize, maxDepth, gridSize int,
	onUpdate func(),
	onComplete func(image.Point),
	logger *zap.Logger,
) *Manager {
	// Use default grid size if invalid (< 2)
	if gridSize < GridSize2x2 {
		logger.Warn("Invalid grid size, using default 2x2",
			zap.Int("provided", gridSize))
		gridSize = GridSize2x2
	}

	// Use default keys if not provided
	if strings.TrimSpace(keys) == "" {
		keys = DefaultKeys
	}

	// Ensure we have the correct number of keys based on grid size
	expectedKeyCount := gridSize * gridSize
	if utf8.RuneCountInString(keys) != expectedKeyCount {
		logger.Warn("Invalid key mapping length, using default",
			zap.String("provided", keys),
			zap.Int("length", utf8.RuneCountInString(keys)),
			zap.Int("expected", expectedKeyCount))
		keys = DefaultKeys
		gridSize = GridSize2x2
	}

	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		grid:       NewRecursiveGridWithSize(screenBounds, minSize, maxDepth, gridSize),
		keys:       strings.ToLower(keys),
		gridSize:   gridSize,
		onUpdate:   onUpdate,
		onComplete: onComplete,
		resetKey:   resetKey,
		exitKeys:   exitKeys,
	}
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
			m.Logger.Debug("Exit key pressed in recursive-grid mode",
				zap.String("key", key))

			return image.Point{}, false, true
		}
	}

	// Handle reset key
	if config.IsResetKey(key, m.resetKey) {
		m.Logger.Debug("Reset key pressed in recursive-grid mode",
			zap.String("key", key))
		m.Reset()

		if m.onUpdate != nil {
			m.onUpdate()
		}

		// Return initial center to move cursor back
		return m.grid.CurrentCenter(), false, false
	}

	// Handle backspace/delete for backtracking
	if config.IsBackspaceKey(key) {
		if m.grid.Backtrack() {
			m.Logger.Debug("Backtracked in recursive-grid mode",
				zap.Int("new_depth", m.grid.CurrentDepth()))

			if m.onUpdate != nil {
				m.onUpdate()
			}

			// Return new center (of parent cell) to move cursor
			return m.grid.CurrentCenter(), false, false
		}

		return image.Point{}, false, false
	}

	// Map key to cell
	cell := m.keyToCell(key)
	if cell < 0 {
		// Key not mapped to any cell
		m.Logger.Debug("Unmapped key pressed in recursive-grid mode",
			zap.String("key", key))

		return image.Point{}, false, false
	}

	// Select the cell
	center, isComplete := m.grid.SelectCell(cell)

	m.Logger.Debug("Cell selected",
		zap.String("key", key),
		zap.Int("cell", int(cell)),
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

// CurrentGrid returns the underlying RecursiveGrid instance.
func (m *Manager) CurrentGrid() *RecursiveGrid {
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

// CellCenter returns the center point for a specific cell.
func (m *Manager) CellCenter(q Cell) image.Point {
	return m.grid.CellCenter(q)
}

// CellBounds returns the bounds for a specific cell.
func (m *Manager) CellBounds(q Cell) image.Rectangle {
	return m.grid.CellBounds(q)
}

// Keys returns the current key mapping.
func (m *Manager) Keys() string {
	return m.keys
}

// GridSize returns the current grid size (2 for 2x2, 3 for 3x3).
func (m *Manager) GridSize() int {
	return m.gridSize
}

// UpdateKeys updates the key mapping.
func (m *Manager) UpdateKeys(keys string) {
	expectedKeyCount := m.gridSize * m.gridSize
	if utf8.RuneCountInString(keys) == expectedKeyCount {
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

// keyToCell maps an input key to a cell index.
// Returns -1 if the key is not mapped.
func (m *Manager) keyToCell(key string) Cell {
	idx := 0
	for _, k := range m.keys {
		if string(k) == key {
			return Cell(idx)
		}

		idx++
	}

	return -1
}
