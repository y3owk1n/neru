package recursivegrid

import (
	"image"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// Manager handles recursive-grid navigation state and input processing.
type Manager struct {
	domain.BaseManager

	grid      *RecursiveGrid
	keys      string                // Default key mapping (e.g., "rtyfghvbn")
	depthKeys map[int]string        // Per-depth key overrides (sparse)
	dims      domain.GridDimensions // Default shape at depths with no override
	callbacks SelectionCallbacks    // What the selection tells its owner
}

// NewManager creates a recursive-grid manager with default dimensions (3×3)
// and default size/depth limits. Used primarily in tests.
func NewManager(
	screenBounds image.Rectangle,
	keys string,
	callbacks SelectionCallbacks,
	logger *zap.Logger,
) *Manager {
	return NewManagerWithLayers(
		screenBounds,
		keys,
		25, //nolint:mnd
		25, //nolint:mnd
		10, //nolint:mnd
		DefaultDimensions(),
		nil, nil,
		callbacks,
		logger,
	)
}

// NewManagerWithLayers creates a manager with a custom shape and optional
// per-depth layout and key overrides. Pass nil for depthLayouts/depthKeys
// to use default dimensions at all depths.
//
// dims arrives as one value rather than a column count beside a row count so
// that a caller cannot hand over the two in the wrong order (#1313), and
// callbacks arrives as one value for the same reason (#1346).
func NewManagerWithLayers(
	screenBounds image.Rectangle,
	keys string,
	minSizeWidth, minSizeHeight, maxDepth int,
	dims domain.GridDimensions,
	depthLayouts map[int]DepthLayout,
	depthKeys map[int]string,
	callbacks SelectionCallbacks,
	logger *zap.Logger,
) *Manager {
	// A shape that cannot narrow anything is replaced with the default one.
	// UsableDimensions owns that rule; what belongs here is naming the values
	// it rejected, which it cannot report once it has replaced them.
	usableDims, asGiven := UsableDimensions(dims)
	if !asGiven {
		logger.Warn("Invalid grid dimensions, using default",
			zap.Int("provided_cols", dims.Cols),
			zap.Int("provided_rows", dims.Rows))
	}

	dims = usableDims

	// Use default keys if not provided
	if strings.TrimSpace(keys) == "" {
		keys = DefaultKeys
	}

	// Ensure we have the correct number of keys based on grid dimensions
	expectedKeyCount := dims.CellCount()
	if utf8.RuneCountInString(keys) != expectedKeyCount {
		logger.Warn("Invalid key mapping length, using default",
			zap.String("provided", keys),
			zap.Int("length", utf8.RuneCountInString(keys)),
			zap.Int("expected", expectedKeyCount))
		keys = DefaultKeys
		dims = DefaultDimensions()
	}

	if depthKeys == nil {
		depthKeys = make(map[int]string)
	}

	if depthLayouts == nil {
		depthLayouts = make(map[int]DepthLayout)
	}

	// Validate consistency: every depth that appears in one map must appear
	// in the other, and the key count must match the layout dimensions.
	// Mismatched entries are dropped with a warning to prevent keyToCell
	// returning cell indices outside the range of Divide().
	for depth := range depthLayouts {
		depthKey, hasKeys := depthKeys[depth]
		if !hasKeys {
			logger.Warn(
				"depthLayouts has depth with no matching depthKeys entry; dropping override",
				zap.Int("depth", depth),
			)
			delete(depthLayouts, depth)

			continue
		}

		expected := depthLayouts[depth].GridCols * depthLayouts[depth].GridRows
		if utf8.RuneCountInString(depthKey) != expected {
			logger.Warn(
				"depthKeys length does not match depthLayouts dimensions; dropping override",
				zap.Int("depth", depth),
				zap.Int("expected_keys", expected),
				zap.Int("actual_keys", utf8.RuneCountInString(depthKey)),
			)
			delete(depthLayouts, depth)
			delete(depthKeys, depth)
		}
	}

	for depth := range depthKeys {
		if _, hasLayout := depthLayouts[depth]; !hasLayout {
			logger.Warn(
				"depthKeys has depth with no matching depthLayouts entry; dropping override",
				zap.Int("depth", depth),
			)
			delete(depthKeys, depth)
		}
	}

	// Normalize all depth keys to lowercase
	normalizedDepthKeys := make(map[int]string, len(depthKeys))
	for depth, dk := range depthKeys {
		normalizedDepthKeys[depth] = strings.ToLower(dk)
	}

	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		grid: NewRecursiveGridWithLayers(
			screenBounds,
			minSizeWidth,
			minSizeHeight,
			maxDepth,
			dims,
			depthLayouts,
		),
		keys:      strings.ToLower(keys),
		depthKeys: normalizedDepthKeys,
		dims:      dims,
		callbacks: callbacks,
	}
}

// HandleInput processes a key press and updates the grid state.
// Returns the new cursor position (if applicable) and whether the selection is complete.
func (m *Manager) HandleInput(key string) (image.Point, bool) {
	// Normalize to canonical key form (handles named keys and fullwidth input),
	// then lowercase for case-insensitive comparisons.
	key = strings.ToLower(configpkg.NormalizeKeyForComparison(key))

	cell := m.keyToCell(key)
	if cell < 0 {
		// Key not mapped to any cell
		m.Logger.Debug("Unmapped key pressed in recursive-grid mode",
			zap.String("key", key))

		return image.Point{}, false
	}

	center, isComplete := m.grid.SelectCell(cell)

	m.Logger.Debug("Cell selected",
		zap.String("key", key),
		zap.Int("cell", int(cell)),
		zap.Int("depth", m.grid.CurrentDepth()),
		zap.Bool("complete", isComplete),
		zap.Int("center_x", center.X),
		zap.Int("center_y", center.Y))

	// If complete, call the completion callback and skip the overlay update
	// because SelectCell returns early without changing bounds/depth.
	if isComplete {
		if m.callbacks.OnComplete != nil {
			m.callbacks.OnComplete(center)
		}

		return center, isComplete
	}

	// Trigger update for visual feedback
	if m.callbacks.OnUpdate != nil {
		m.callbacks.OnUpdate(center)
	}

	return center, isComplete
}

// MoveDirection slides the current selection count cells in dir without
// changing depth, and fires the update callback when the selection changed.
// Returns the resulting selection center and whether anything moved.
func (m *Manager) MoveDirection(dir domain.Direction, count int) (image.Point, bool) {
	center, moved := m.grid.MoveDirection(dir, count)

	m.Logger.Debug("Recursive-grid directional move",
		zap.String("direction", dir.String()),
		zap.Int("count", count),
		zap.Int("depth", m.grid.CurrentDepth()),
		zap.Bool("moved", moved))

	if !moved {
		return center, false
	}

	if m.callbacks.OnUpdate != nil {
		m.callbacks.OnUpdate(center)
	}

	return center, true
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

// IsComplete returns true if the grid cannot be divided further (min size or max depth).
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

// Keys returns the key mapping for the current depth.
func (m *Manager) Keys() string {
	return m.KeysForDepth(m.grid.CurrentDepth())
}

// KeysForDepth returns the key mapping for the given depth.
func (m *Manager) KeysForDepth(depth int) string {
	if dk, ok := m.depthKeys[depth]; ok {
		return dk
	}

	return m.keys
}

// GridCols returns the number of grid columns for the current depth.
func (m *Manager) GridCols() int {
	return m.grid.GridCols()
}

// GridRows returns the number of grid rows for the current depth.
func (m *Manager) GridRows() int {
	return m.grid.GridRows()
}

// UpdateKeys updates the default key mapping.
func (m *Manager) UpdateKeys(keys string) {
	expectedKeyCount := m.dims.CellCount()
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

// ZoomToPoint auto-navigates to the given target depth based on cursor position.
// Returns the final center point and whether the selection is complete.
func (m *Manager) ZoomToPoint(point image.Point, targetDepth int) (image.Point, bool) {
	center, isComplete := m.grid.ZoomToPoint(point, targetDepth)

	m.SetCurrentInput("")

	if isComplete {
		if m.callbacks.OnComplete != nil {
			m.callbacks.OnComplete(center)
		}

		return center, true
	}

	if m.callbacks.OnUpdate != nil {
		m.callbacks.OnUpdate(center)
	}

	return center, false
}

// keyToCell maps an input key to a cell index using the current depth's key mapping.
// Returns -1 if the key is not mapped.
func (m *Manager) keyToCell(key string) Cell {
	currentKeys := m.Keys()
	idx := 0

	for _, k := range currentKeys {
		normalizedMapped := strings.ToLower(configpkg.NormalizeKeyForComparison(string(k)))
		if strings.EqualFold(normalizedMapped, key) {
			return Cell(idx)
		}

		idx++
	}

	return -1
}
