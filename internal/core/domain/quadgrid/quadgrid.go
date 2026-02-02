package quadgrid

import (
	"image"
)

// Quadrant represents one of the four screen divisions.
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
	history       []image.Rectangle // Stack of previous bounds for backtracking
}

// NewQuadGrid creates a new quad-grid starting with the given screen bounds.
func NewQuadGrid(screenBounds image.Rectangle, minSize, maxDepth int) *QuadGrid {
	return &QuadGrid{
		currentBounds: screenBounds,
		initialBounds: screenBounds,
		depth:         0,
		maxDepth:      maxDepth,
		minSize:       minSize,
		history:       make([]image.Rectangle, 0, maxDepth),
	}
}

// Divide splits the current bounds into 4 equal quadrants.
// Returns an array where index corresponds to Quadrant enum values.
func (qg *QuadGrid) Divide() [4]image.Rectangle {
	midX := qg.currentBounds.Min.X + qg.currentBounds.Dx()/2
	midY := qg.currentBounds.Min.Y + qg.currentBounds.Dy()/2

	return [4]image.Rectangle{
		TopLeft: image.Rect(
			qg.currentBounds.Min.X, qg.currentBounds.Min.Y,
			midX, midY,
		),
		TopRight: image.Rect(
			midX, qg.currentBounds.Min.Y,
			qg.currentBounds.Max.X, midY,
		),
		BottomLeft: image.Rect(
			qg.currentBounds.Min.X, midY,
			midX, qg.currentBounds.Max.Y,
		),
		BottomRight: image.Rect(
			midX, midY,
			qg.currentBounds.Max.X, qg.currentBounds.Max.Y,
		),
	}
}

// SelectQuadrant narrows the active area to the selected quadrant.
// Returns the center point of the selected quadrant and whether the selection is complete.
// Selection is complete when the minimum size is reached.
func (qg *QuadGrid) SelectQuadrant(q Quadrant) (image.Point, bool) {
	// Check if we can divide further
	if !qg.CanDivide() {
		// If we can't divide further (max depth or min size),
		// return the center of the selected quadrant without changing bounds.
		quadrants := qg.Divide()
		selected := quadrants[q]
		center := image.Point{
			X: selected.Min.X + selected.Dx()/2,
			Y: selected.Min.Y + selected.Dy()/2,
		}
		return center, true
	}

	quadrants := qg.Divide()
	selected := quadrants[q]

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

	// Check size constraints - both dimensions must be divisible by 2
	// and the result must be >= minSize
	halfWidth := qg.currentBounds.Dx() / 2
	halfHeight := qg.currentBounds.Dy() / 2

	return halfWidth >= qg.minSize && halfHeight >= qg.minSize
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

	return quadrants[q]
}

// QuadrantCenter returns the center point for a specific quadrant without selecting it.
// Useful for visual rendering and cursor preview.
func (qg *QuadGrid) QuadrantCenter(q Quadrant) image.Point {
	bounds := qg.QuadrantBounds(q)

	return image.Point{
		X: bounds.Min.X + bounds.Dx()/2,
		Y: bounds.Min.Y + bounds.Dy()/2,
	}
}
