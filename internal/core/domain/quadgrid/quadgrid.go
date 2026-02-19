package quadgrid

import (
	"image"
)

const (
	// GridSize2x2 represents the default 2x2 grid layout.
	GridSize2x2 = 2
)

// Quadrant represents one of the screen divisions.
// For 2x2: 0=TL, 1=TR, 2=BL, 3=BR.
// For 3x3: 0-8 left-to-right, top-to-bottom.
type Quadrant int

const (
	// TopLeft represents the upper-left quadrant (default key: 'u').
	TopLeft Quadrant = iota
	// TopRight represents the upper-right quadrant (default key: 'i').
	TopRight
	// BottomLeft represents the lower-left quadrant (default key: 'j').
	BottomLeft
	// BottomRight represents the lower-right quadrant (default key: 'k').
	BottomRight
)

// DefaultKeys is the default key mapping for quadrants (warpd convention).
const DefaultKeys = "uijk"

// QuadGrid represents the recursive grid state for quadrant-based navigation.
type QuadGrid struct {
	currentBounds image.Rectangle   // Current active area
	initialBounds image.Rectangle   // Original screen bounds
	depth         int               // Current recursion depth
	maxDepth      int               // Maximum allowed depth
	minSize       int               // Minimum quadrant size in pixels
	gridSize      int               // Grid size: 2 for 2x2, 3 for 3x3
	history       []image.Rectangle // Stack of previous bounds for backtracking
}

// NewQuadGrid creates a new quad-grid starting with the given screen bounds.
func NewQuadGrid(screenBounds image.Rectangle, minSize, maxDepth int) *QuadGrid {
	return NewQuadGridWithSize(screenBounds, minSize, maxDepth, GridSize2x2)
}

// NewQuadGridWithSize creates a new quad-grid with a specific grid size.
func NewQuadGridWithSize(screenBounds image.Rectangle, minSize, maxDepth, gridSize int) *QuadGrid {
	return &QuadGrid{
		currentBounds: screenBounds,
		initialBounds: screenBounds,
		depth:         0,
		maxDepth:      maxDepth,
		minSize:       minSize,
		gridSize:      gridSize,
		history:       make([]image.Rectangle, 0, maxDepth),
	}
}

// GridSize returns the grid size (2 for 2x2, 3 for 3x3).
func (qg *QuadGrid) GridSize() int {
	return qg.gridSize
}

// Divide splits the current bounds into quadrants based on grid size.
// For 2x2: returns 4 quadrants (TL, TR, BL, BR).
// For 3x3: returns 9 quadrants (left-to-right, top-to-bottom).
func (qg *QuadGrid) Divide() []image.Rectangle {
	cellWidth := qg.currentBounds.Dx() / qg.gridSize
	cellHeight := qg.currentBounds.Dy() / qg.gridSize

	quadrants := make([]image.Rectangle, qg.gridSize*qg.gridSize)

	for row := range qg.gridSize {
		for col := range qg.gridSize {
			idx := row*qg.gridSize + col
			quadrants[idx] = image.Rect(
				qg.currentBounds.Min.X+col*cellWidth,
				qg.currentBounds.Min.Y+row*cellHeight,
				qg.currentBounds.Min.X+(col+1)*cellWidth,
				qg.currentBounds.Min.Y+(row+1)*cellHeight,
			)
		}
	}

	return quadrants
}

// QuadrantCenter returns the center point of the specified quadrant.
func (qg *QuadGrid) QuadrantCenter(quadrant Quadrant) image.Point {
	quadrants := qg.Divide()
	idx := int(quadrant)

	if idx >= len(quadrants) {
		return qg.CurrentCenter()
	}

	selected := quadrants[idx]

	return image.Point{
		X: selected.Min.X + selected.Dx()/2,
		Y: selected.Min.Y + selected.Dy()/2,
	}
}

// SelectQuadrant narrows the active area to the selected quadrant.
// Returns the center point of the selected quadrant and whether the selection is complete.
// Selection is complete when the minimum size is reached.
func (qg *QuadGrid) SelectQuadrant(quadrant Quadrant) (image.Point, bool) {
	// Check if we can divide further
	if !qg.CanDivide() {
		// If we can't divide further (max depth or min size),
		// return the center of the selected quadrant without changing bounds.
		return qg.QuadrantCenter(quadrant), true
	}

	quadrants := qg.Divide()
	selected := quadrants[quadrant]

	// Save current bounds for backtracking
	qg.history = append(qg.history, qg.currentBounds)
	qg.currentBounds = selected
	qg.depth++

	center := image.Point{
		X: selected.Min.X + selected.Dx()/2,
		Y: selected.Min.Y + selected.Dy()/2,
	}

	// Check if we've reached minimum size after this selection
	if !qg.CanDivide() {
		return center, true
	}

	return center, false
}

// CanDivide checks if the current bounds can be divided further.
// Returns false when the quadrant would be smaller than minSize or maxDepth is reached.
func (qg *QuadGrid) CanDivide() bool {
	// Check depth limit
	if qg.depth >= qg.maxDepth {
		return false
	}

	// Check size constraints - both dimensions must be divisible by gridSize
	// and the result must be >= minSize
	cellWidth := qg.currentBounds.Dx() / qg.gridSize
	cellHeight := qg.currentBounds.Dy() / qg.gridSize

	return cellWidth >= qg.minSize && cellHeight >= qg.minSize
}

// CurrentCenter returns the center point of the current bounds.
func (qg *QuadGrid) CurrentCenter() image.Point {
	return image.Point{
		X: qg.currentBounds.Min.X + qg.currentBounds.Dx()/2,
		Y: qg.currentBounds.Min.Y + qg.currentBounds.Dy()/2,
	}
}

// CurrentBounds returns the current active bounds.
func (qg *QuadGrid) CurrentBounds() image.Rectangle {
	return qg.currentBounds
}

// InitialBounds returns the original screen bounds.
func (qg *QuadGrid) InitialBounds() image.Rectangle {
	return qg.initialBounds
}

// CurrentDepth returns the current recursion depth.
func (qg *QuadGrid) CurrentDepth() int {
	return qg.depth
}

// MaxDepth returns the maximum allowed recursion depth.
func (qg *QuadGrid) MaxDepth() int {
	return qg.maxDepth
}

// MinSize returns the minimum quadrant size.
func (qg *QuadGrid) MinSize() int {
	return qg.minSize
}

// Backtrack returns to the previous bounds (undo last selection).
// Returns true if backtracking was successful, false if there's no history.
func (qg *QuadGrid) Backtrack() bool {
	if len(qg.history) == 0 {
		return false
	}

	// Pop last bounds from history
	lastIndex := len(qg.history) - 1
	qg.currentBounds = qg.history[lastIndex]
	qg.history = qg.history[:lastIndex]
	qg.depth--

	return true
}

// HasHistory returns true if there's backtrack history available.
func (qg *QuadGrid) HasHistory() bool {
	return len(qg.history) > 0
}

// Reset restores the grid to its initial state.
func (qg *QuadGrid) Reset() {
	qg.currentBounds = qg.initialBounds
	qg.depth = 0
	qg.history = qg.history[:0]
}

// IsComplete returns true if the minimum size has been reached.
func (qg *QuadGrid) IsComplete() bool {
	return !qg.CanDivide()
}

// QuadrantBounds returns the bounds for a specific quadrant without selecting it.
// Useful for visual rendering.
func (qg *QuadGrid) QuadrantBounds(q Quadrant) image.Rectangle {
	quadrants := qg.Divide()
	idx := int(q)

	if idx >= len(quadrants) {
		return qg.currentBounds
	}

	return quadrants[idx]
}
