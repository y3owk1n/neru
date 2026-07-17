package recursivegrid

import (
	"image"
)

const (
	// MinGridDimension is the minimum allowed value for grid columns or rows.
	MinGridDimension = 1
	// DefaultGridCols is the default recursive-grid column count.
	DefaultGridCols = 3
	// DefaultGridRows is the default recursive-grid row count.
	DefaultGridRows = 3
	// CenterDivisor is the divisor used when computing the center of a rectangle,
	// i.e. Min.X + Dx()/CenterDivisor.
	CenterDivisor = 2
)

// Cell represents the index of a cell in the grid.
// For 3x3 grids: 0=TL, 1=TM, 2=TR, 3=ML, 4=MM, 5=MR, 6=BL, 7=BM, 8=BR (named constants below are only for 2x2).
// For CxR grids: indices 0 to (C*R-1) are ordered left-to-right, top-to-bottom.
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

// DefaultKeys is the default key mapping for cells (3x3 grid, left-to-right top-to-bottom).
const DefaultKeys = "rtyfghvbn"

// DepthLayout defines the grid dimensions for a specific recursion depth.
type DepthLayout struct {
	GridCols int
	GridRows int
}

// RecursiveGrid represents the recursive grid state for cell-based navigation.
type RecursiveGrid struct {
	currentBounds image.Rectangle     // Current active area
	initialBounds image.Rectangle     // Original screen bounds
	depth         int                 // Current recursion depth
	maxDepth      int                 // Maximum allowed depth
	minSizeWidth  int                 // Minimum cell width in pixels
	minSizeHeight int                 // Minimum cell height in pixels
	gridCols      int                 // Default number of grid columns
	gridRows      int                 // Default number of grid rows
	depthLayouts  map[int]DepthLayout // Per-depth layout overrides (sparse)
	history       []image.Rectangle   // Stack of previous bounds for backtracking
}

// NewRecursiveGrid creates a new recursive-grid starting with the given screen bounds
// using the default 3×3 grid dimensions and no per-depth overrides.
func NewRecursiveGrid(
	screenBounds image.Rectangle,
	minSizeWidth,
	minSizeHeight,
	maxDepth int,
) *RecursiveGrid {
	return NewRecursiveGridWithLayers(
		screenBounds,
		minSizeWidth, minSizeHeight, maxDepth,
		DefaultGridCols, DefaultGridRows,
		nil,
	)
}

// NewRecursiveGridWithLayers creates a new recursive-grid with specific column/row counts
// and optional per-depth layout overrides. Pass nil for depthLayouts to use default dimensions at all depths.
func NewRecursiveGridWithLayers(
	screenBounds image.Rectangle,
	minSizeWidth, minSizeHeight, maxDepth, gridCols, gridRows int,
	depthLayouts map[int]DepthLayout,
) *RecursiveGrid {
	if depthLayouts == nil {
		depthLayouts = make(map[int]DepthLayout)
	}

	return &RecursiveGrid{
		currentBounds: screenBounds,
		initialBounds: screenBounds,
		depth:         0,
		maxDepth:      maxDepth,
		minSizeWidth:  minSizeWidth,
		minSizeHeight: minSizeHeight,
		gridCols:      gridCols,
		gridRows:      gridRows,
		depthLayouts:  depthLayouts,
		history:       make([]image.Rectangle, 0, maxDepth),
	}
}

// LayoutForDepth returns the grid dimensions for the given depth.
// If a per-depth override exists, it is returned; otherwise the defaults are used.
func (qg *RecursiveGrid) LayoutForDepth(depth int) DepthLayout {
	if layout, ok := qg.depthLayouts[depth]; ok {
		return layout
	}

	return DepthLayout{GridCols: qg.gridCols, GridRows: qg.gridRows}
}

// GridCols returns the number of grid columns for the current depth.
func (qg *RecursiveGrid) GridCols() int {
	return qg.LayoutForDepth(qg.depth).GridCols
}

// GridRows returns the number of grid rows for the current depth.
func (qg *RecursiveGrid) GridRows() int {
	return qg.LayoutForDepth(qg.depth).GridRows
}

// Divide splits the current bounds into cells based on grid dimensions for the current depth.
// Cells are ordered left-to-right, top-to-bottom.
// Every cell is exactly the same size: floor(W/cols) × floor(H/rows). Any leftover pixels
// from uneven division are distributed as uniform padding around the grid, so cells within
// a single level are always identically sized.
func (qg *RecursiveGrid) Divide() []image.Rectangle {
	layout := qg.LayoutForDepth(qg.depth)
	cols := layout.GridCols
	rows := layout.GridRows
	boundsW := qg.currentBounds.Dx()
	boundsH := qg.currentBounds.Dy()
	cellW := boundsW / cols
	cellH := boundsH / rows
	usedW := cellW * cols
	usedH := cellH * rows
	offX := (boundsW - usedW) / CenterDivisor
	offY := (boundsH - usedH) / CenterDivisor

	cells := make([]image.Rectangle, cols*rows)
	for row := range rows {
		for col := range cols {
			idx := row*cols + col
			x := qg.currentBounds.Min.X + offX + col*cellW
			y := qg.currentBounds.Min.Y + offY + row*cellH
			cells[idx] = image.Rect(x, y, x+cellW, y+cellH)
		}
	}

	return cells
}

// CellCenter returns the center point of the specified cell, rounded to nearest pixel.
func (qg *RecursiveGrid) CellCenter(cell Cell) image.Point {
	cells := qg.Divide()
	idx := int(cell)

	if idx < 0 || idx >= len(cells) {
		return qg.CurrentCenter()
	}

	selected := cells[idx]

	return image.Point{
		X: selected.Min.X + divRound(selected.Dx(), CenterDivisor),
		Y: selected.Min.Y + divRound(selected.Dy(), CenterDivisor),
	}
}

// SelectCell narrows the active area to the selected cell.
// Returns the center point of the selected cell and whether the selection is complete.
// When the grid cannot be divided further (min size or max depth), the selection completes
// and currentBounds is left unchanged so that backtrack restores the correct ancestor bounds.
func (qg *RecursiveGrid) SelectCell(cell Cell) (image.Point, bool) {
	cells := qg.Divide()
	idx := int(cell)

	// Bounds check - return center of current bounds for invalid cell
	if idx >= len(cells) || idx < 0 {
		return qg.CurrentCenter(), true
	}

	selected := cells[idx]

	// Compute center from the cell bounds before any state mutation.
	center := image.Point{
		X: selected.Min.X + divRound(selected.Dx(), CenterDivisor),
		Y: selected.Min.Y + divRound(selected.Dy(), CenterDivisor),
	}

	if !qg.CanDivide() {
		return center, true
	}

	// Save current bounds for backtracking
	qg.history = append(qg.history, qg.currentBounds)
	qg.currentBounds = selected
	qg.depth++

	return center, false
}

// CanDivide checks if the current bounds can be divided further.
// Returns false when the cell would be smaller than minSize or maxDepth is reached.
func (qg *RecursiveGrid) CanDivide() bool {
	// Check depth limit
	if qg.depth >= qg.maxDepth {
		return false
	}

	// Check size constraints using the layout for the current depth
	layout := qg.LayoutForDepth(qg.depth)
	cellWidth := qg.currentBounds.Dx() / layout.GridCols
	cellHeight := qg.currentBounds.Dy() / layout.GridRows

	return cellWidth >= qg.minSizeWidth && cellHeight >= qg.minSizeHeight
}

// CurrentCenter returns the center point of the current bounds, rounded to nearest pixel.
func (qg *RecursiveGrid) CurrentCenter() image.Point {
	return image.Point{
		X: qg.currentBounds.Min.X + divRound(qg.currentBounds.Dx(), CenterDivisor),
		Y: qg.currentBounds.Min.Y + divRound(qg.currentBounds.Dy(), CenterDivisor),
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

// MinSizeWidth returns the minimum cell width.
func (qg *RecursiveGrid) MinSizeWidth() int {
	return qg.minSizeWidth
}

// MinSizeHeight returns the minimum cell height.
func (qg *RecursiveGrid) MinSizeHeight() int {
	return qg.minSizeHeight
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

// RemapToNewBounds proportionally remaps all bounds (history + currentBounds)
// from the old initial bounds to newBounds, preserving the user's depth and
// selection progress. This is used during screen changes so the zoomed-in
// region maps to the equivalent proportional area on the new screen.
func (qg *RecursiveGrid) RemapToNewBounds(newBounds image.Rectangle) {
	oldInitial := qg.initialBounds
	for i, h := range qg.history {
		qg.history[i] = remapRect(h, oldInitial, newBounds)
	}

	qg.currentBounds = remapRect(qg.currentBounds, oldInitial, newBounds)
	qg.initialBounds = newBounds
}

// remapRect proportionally maps r from the coordinate space of oldRef into newRef.
// It uses rounded integer division to minimize per-remap error, reducing drift
// when multiple successive screen changes occur.
func remapRect(rect, oldRef, newRef image.Rectangle) image.Rectangle {
	oldW := oldRef.Dx()
	oldH := oldRef.Dy()

	if oldW == 0 || oldH == 0 {
		return newRef
	}

	// Express rect's edges as fractions of oldRef, then scale to newRef.
	// Use divRound for rounding to nearest instead of truncation, which
	// halves the maximum per-remap error (~0.5px vs ~1px) and reduces
	// cumulative drift across successive screen changes.
	minX := newRef.Min.X + divRound((rect.Min.X-oldRef.Min.X)*newRef.Dx(), oldW)
	minY := newRef.Min.Y + divRound((rect.Min.Y-oldRef.Min.Y)*newRef.Dy(), oldH)
	maxX := newRef.Min.X + divRound((rect.Max.X-oldRef.Min.X)*newRef.Dx(), oldW)
	maxY := newRef.Min.Y + divRound((rect.Max.Y-oldRef.Min.Y)*newRef.Dy(), oldH)

	return image.Rect(minX, minY, maxX, maxY)
}

// divRound performs integer division of numerator by denominator, rounding to
// the nearest integer instead of truncating toward zero.
func divRound(numerator, denominator int) int {
	if denominator == 0 {
		return 0
	}

	// Handle negative results correctly: round toward nearest, not toward zero.
	if (numerator < 0) != (denominator < 0) {
		return (numerator - denominator/2) / denominator
	}

	return (numerator + denominator/2) / denominator
}

// IsComplete returns true if the grid cannot be divided further (min size or max depth).
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
