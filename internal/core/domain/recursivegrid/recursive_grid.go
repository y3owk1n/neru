package recursivegrid

import (
	"image"
)

const (
	// GridSize2x2 represents the default 2x2 grid layout.
	GridSize2x2 = 2
)

// Cell represents the index of a cell in the grid.
// For 2x2 grids: 0=TL, 1=TR, 2=BL, 3=BR (named constants below).
// For NxN grids: indices 0 to (N*N-1) are ordered left-to-right, top-to-bottom.
// The named constants (TopLeft, TopRight, etc.) are only meaningful for 2x2 grids.
type Cell int

const (
	// TopLeft represents the upper-left cell in a 2x2 grid (index 0).
	TopLeft Cell = iota
	// TopRight represents the upper-right cell in a 2x2 grid (index 1).
	TopRight
	// BottomLeft represents the lower-left cell in a 2x2 grid (index 2).
	BottomLeft
	// BottomRight represents the lower-right cell in a 2x2 grid (index 3).
	BottomRight
)

// DefaultKeys is the default key mapping for cells (warpd convention).
const DefaultKeys = "uijk"

// RecursiveGrid represents the recursive grid state for cell-based navigation.
type RecursiveGrid struct {
	currentBounds image.Rectangle   // Current active area
	initialBounds image.Rectangle   // Original screen bounds
	depth         int               // Current recursion depth
	maxDepth      int               // Maximum allowed depth
	minSize       int               // Minimum cell size in pixels
	gridSize      int               // Grid size: 2 for 2x2, 3 for 3x3
	history       []image.Rectangle // Stack of previous bounds for backtracking
}

// NewRecursiveGrid creates a new recursive-grid starting with the given screen bounds.
func NewRecursiveGrid(screenBounds image.Rectangle, minSize, maxDepth int) *RecursiveGrid {
	return NewRecursiveGridWithSize(screenBounds, minSize, maxDepth, GridSize2x2)
}

// NewRecursiveGridWithSize creates a new recursive-grid with a specific grid size.
func NewRecursiveGridWithSize(
	screenBounds image.Rectangle,
	minSize, maxDepth, gridSize int,
) *RecursiveGrid {
	return &RecursiveGrid{
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
func (qg *RecursiveGrid) GridSize() int {
	return qg.gridSize
}

// Divide splits the current bounds into cells based on grid size.
// For 2x2: returns 4 cells (TL, TR, BL, BR).
// For 3x3: returns 9 cells (left-to-right, top-to-bottom).
func (qg *RecursiveGrid) Divide() []image.Rectangle {
	cellWidth := qg.currentBounds.Dx() / qg.gridSize
	cellHeight := qg.currentBounds.Dy() / qg.gridSize

	cells := make([]image.Rectangle, qg.gridSize*qg.gridSize)

	for row := range qg.gridSize {
		for col := range qg.gridSize {
			idx := row*qg.gridSize + col

			maxX := qg.currentBounds.Min.X + (col+1)*cellWidth
			if col == qg.gridSize-1 {
				maxX = qg.currentBounds.Max.X
			}

			maxY := qg.currentBounds.Min.Y + (row+1)*cellHeight
			if row == qg.gridSize-1 {
				maxY = qg.currentBounds.Max.Y
			}

			cells[idx] = image.Rect(
				qg.currentBounds.Min.X+col*cellWidth,
				qg.currentBounds.Min.Y+row*cellHeight,
				maxX,
				maxY,
			)
		}
	}

	return cells
}

// CellCenter returns the center point of the specified cell.
func (qg *RecursiveGrid) CellCenter(cell Cell) image.Point {
	cells := qg.Divide()
	idx := int(cell)

	if idx < 0 || idx >= len(cells) {
		return qg.CurrentCenter()
	}

	selected := cells[idx]

	return image.Point{
		X: selected.Min.X + selected.Dx()/2,
		Y: selected.Min.Y + selected.Dy()/2,
	}
}

// SelectCell narrows the active area to the selected cell.
// Returns the center point of the selected cell and whether the selection is complete.
// Selection is complete when the minimum size is reached.
func (qg *RecursiveGrid) SelectCell(cell Cell) (image.Point, bool) {
	cells := qg.Divide()
	idx := int(cell)

	// Bounds check - return center of current bounds for invalid cell
	if idx >= len(cells) || idx < 0 {
		return qg.CurrentCenter(), true
	}

	// Check if we can divide further
	if !qg.CanDivide() {
		// If we can't divide further (max depth or min size),
		// return the center of the selected cell without changing bounds.
		return qg.CellCenter(cell), true
	}

	selected := cells[idx]

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
// Returns false when the cell would be smaller than minSize or maxDepth is reached.
func (qg *RecursiveGrid) CanDivide() bool {
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
func (qg *RecursiveGrid) CurrentCenter() image.Point {
	return image.Point{
		X: qg.currentBounds.Min.X + qg.currentBounds.Dx()/2,
		Y: qg.currentBounds.Min.Y + qg.currentBounds.Dy()/2,
	}
}

// CurrentBounds returns the current active bounds.
func (qg *RecursiveGrid) CurrentBounds() image.Rectangle {
	return qg.currentBounds
}

// InitialBounds returns the original screen bounds.
func (qg *RecursiveGrid) InitialBounds() image.Rectangle {
	return qg.initialBounds
}

// CurrentDepth returns the current recursion depth.
func (qg *RecursiveGrid) CurrentDepth() int {
	return qg.depth
}

// MaxDepth returns the maximum allowed recursion depth.
func (qg *RecursiveGrid) MaxDepth() int {
	return qg.maxDepth
}

// MinSize returns the minimum cell size.
func (qg *RecursiveGrid) MinSize() int {
	return qg.minSize
}

// Backtrack returns to the previous bounds (undo last selection).
// Returns true if backtracking was successful, false if there's no history.
func (qg *RecursiveGrid) Backtrack() bool {
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
func (qg *RecursiveGrid) HasHistory() bool {
	return len(qg.history) > 0
}

// Reset restores the grid to its initial state.
func (qg *RecursiveGrid) Reset() {
	qg.currentBounds = qg.initialBounds
	qg.depth = 0
	qg.history = qg.history[:0]
}

// IsComplete returns true if the minimum size has been reached.
func (qg *RecursiveGrid) IsComplete() bool {
	return !qg.CanDivide()
}

// CellBounds returns the bounds for a specific cell without selecting it.
// Useful for visual rendering.
func (qg *RecursiveGrid) CellBounds(q Cell) image.Rectangle {
	cells := qg.Divide()
	idx := int(q)

	if idx < 0 || idx >= len(cells) {
		return qg.currentBounds
	}

	return cells[idx]
}
