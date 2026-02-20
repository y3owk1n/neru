package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
)

func TestNewRecursiveGrid(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	assert.Equal(t, bounds, grid.CurrentBounds(), "Current bounds should match initial bounds")
	assert.Equal(t, 0, grid.CurrentDepth(), "Initial depth should be 0")
}

func TestDivide(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	cells := grid.Divide()

	// Verify 4 cells
	assert.Len(t, cells, 4, "Should have 4 cells")

	// Verify top-left cell
	expectedTL := image.Rect(0, 0, 50, 50)
	assert.Equal(
		t,
		expectedTL,
		cells[recursivegrid.TopLeft],
		"Top-left cell should be correct",
	)

	// Verify top-right cell
	expectedTR := image.Rect(50, 0, 100, 50)
	assert.Equal(
		t,
		expectedTR,
		cells[recursivegrid.TopRight],
		"Top-right cell should be correct",
	)

	// Verify bottom-left cell
	expectedBL := image.Rect(0, 50, 50, 100)
	assert.Equal(
		t,
		expectedBL,
		cells[recursivegrid.BottomLeft],
		"Bottom-left cell should be correct",
	)

	// Verify bottom-right cell
	expectedBR := image.Rect(50, 50, 100, 100)
	assert.Equal(
		t,
		expectedBR,
		cells[recursivegrid.BottomRight],
		"Bottom-right cell should be correct",
	)
}

func TestSelectCell(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	// Select top-left
	center, completed := grid.SelectCell(recursivegrid.TopLeft)

	expectedCenter := image.Point{X: 25, Y: 25}
	assert.Equal(t, expectedCenter, center, "Center should be at (25, 25)")
	assert.False(t, completed, "Should not be completed after first selection")

	// Verify depth increased
	assert.Equal(t, 1, grid.CurrentDepth(), "Depth should be 1")

	// Verify bounds narrowed
	expectedBounds := image.Rect(0, 0, 50, 50)
	assert.Equal(
		t,
		expectedBounds,
		grid.CurrentBounds(),
		"Bounds should be narrowed to top-left cell",
	)
}

func TestSelectCellCompletion(t *testing.T) {
	// Create a small grid where one selection reaches minimum size
	// With bounds 50x50 and minSize 25:
	// - First division creates 25x25 cells
	// - 25/2 = 12 < 25, so CanDivide returns false after first selection
	bounds := image.Rect(0, 0, 50, 50)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	// Select top-left - should complete since resulting cell (25x25) cannot be divided further
	center, completed := grid.SelectCell(recursivegrid.TopLeft)

	// After selecting top-left, bounds are (0,0)-(25,25), center is at (12, 12)
	expectedCenter := image.Point{X: 12, Y: 12}
	assert.Equal(t, expectedCenter, center, "Center should be at (12, 12)")
	assert.True(t, completed, "Should be completed when min size is reached")
}

func TestCanDivide(t *testing.T) {
	tests := []struct {
		name     string
		bounds   image.Rectangle
		minSize  int
		maxDepth int
		expected bool
	}{
		{
			name:     "Can divide 100x100 with min 25",
			bounds:   image.Rect(0, 0, 100, 100),
			minSize:  25,
			maxDepth: 10,
			expected: true,
		},
		{
			name:     "Cannot divide 40x40 with min 25",
			bounds:   image.Rect(0, 0, 40, 40),
			minSize:  25,
			maxDepth: 10,
			expected: false, // 40/2 = 20 < 25
		},
		{
			name:     "Can divide 50x50 with min 25",
			bounds:   image.Rect(0, 0, 50, 50),
			minSize:  25,
			maxDepth: 10,
			expected: true, // 50/2 = 25 >= 25
		},
		{
			name:     "Cannot divide when max depth reached",
			bounds:   image.Rect(0, 0, 1000, 1000),
			minSize:  1,
			maxDepth: 0,
			expected: false, // depth starts at 0, max is 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGrid(tt.bounds, tt.minSize, tt.minSize, tt.maxDepth)
			result := grid.CanDivide()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	// Make a selection
	grid.SelectCell(recursivegrid.TopLeft)

	// Backtrack
	result := grid.Backtrack()

	assert.True(t, result, "Backtrack should return true")
	assert.Equal(t, bounds, grid.CurrentBounds(), "Bounds should be restored to original")
	assert.Equal(t, 0, grid.CurrentDepth(), "Depth should be 0")

	// Backtrack again should fail (no history)
	result = grid.Backtrack()
	assert.False(t, result, "Backtrack should return false when no history")
}

func TestReset(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	// Make some selections
	grid.SelectCell(recursivegrid.TopLeft)
	grid.SelectCell(recursivegrid.BottomRight)

	// Reset
	grid.Reset()

	assert.Equal(t, bounds, grid.CurrentBounds(), "Bounds should be restored to initial")
	assert.Equal(t, 0, grid.CurrentDepth(), "Depth should be 0")
}

func TestCurrentCenter(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	center := grid.CurrentCenter()
	expected := image.Point{X: 50, Y: 50}
	assert.Equal(t, expected, center, "Center should be at (50, 50)")

	// Select a cell and check new center
	grid.SelectCell(recursivegrid.TopLeft)
	center = grid.CurrentCenter()
	expected = image.Point{X: 25, Y: 25}
	assert.Equal(t, expected, center, "Center should be at (25, 25) after selecting top-left")
}

func TestCellBounds(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	tl := grid.CellBounds(recursivegrid.TopLeft)
	expected := image.Rect(0, 0, 50, 50)
	assert.Equal(t, expected, tl, "Top-left bounds should be correct")

	br := grid.CellBounds(recursivegrid.BottomRight)
	expected = image.Rect(50, 50, 100, 100)
	assert.Equal(t, expected, br, "Bottom-right bounds should be correct")
}

func TestCellCenter(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	center := grid.CellCenter(recursivegrid.TopRight)
	expected := image.Point{X: 75, Y: 25}
	assert.Equal(t, expected, center, "Top-right center should be at (75, 25)")
}

func TestDivide_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithDimensions(bounds, 10, 10, 10, 3, 2)
	cells := grid.Divide()
	// 3 cols × 2 rows = 6 cells
	assert.Len(t, cells, 6, "Should have 6 cells for 3x2 grid")
	// Cell widths: 120/3 = 40, Cell heights: 100/2 = 50
	// Row 0: cells 0, 1, 2
	assert.Equal(t, image.Rect(0, 0, 40, 50), cells[0], "Cell 0 (row0, col0)")
	assert.Equal(t, image.Rect(40, 0, 80, 50), cells[1], "Cell 1 (row0, col1)")
	assert.Equal(t, image.Rect(80, 0, 120, 50), cells[2], "Cell 2 (row0, col2)")
	// Row 1: cells 3, 4, 5
	assert.Equal(t, image.Rect(0, 50, 40, 100), cells[3], "Cell 3 (row1, col0)")
	assert.Equal(t, image.Rect(40, 50, 80, 100), cells[4], "Cell 4 (row1, col1)")
	assert.Equal(t, image.Rect(80, 50, 120, 100), cells[5], "Cell 5 (row1, col2)")
}

func TestDivide_NonSquare2x3(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 120)
	grid := recursivegrid.NewRecursiveGridWithDimensions(bounds, 10, 10, 10, 2, 3)
	cells := grid.Divide()
	// 2 cols × 3 rows = 6 cells
	assert.Len(t, cells, 6, "Should have 6 cells for 2x3 grid")
	// Cell widths: 100/2 = 50, Cell heights: 120/3 = 40
	// Row 0
	assert.Equal(t, image.Rect(0, 0, 50, 40), cells[0], "Cell 0 (row0, col0)")
	assert.Equal(t, image.Rect(50, 0, 100, 40), cells[1], "Cell 1 (row0, col1)")
	// Row 1
	assert.Equal(t, image.Rect(0, 40, 50, 80), cells[2], "Cell 2 (row1, col0)")
	assert.Equal(t, image.Rect(50, 40, 100, 80), cells[3], "Cell 3 (row1, col1)")
	// Row 2
	assert.Equal(t, image.Rect(0, 80, 50, 120), cells[4], "Cell 4 (row2, col0)")
	assert.Equal(t, image.Rect(50, 80, 100, 120), cells[5], "Cell 5 (row2, col1)")
}

func TestCellCenter_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithDimensions(bounds, 10, 10, 10, 3, 2)
	// Cell 0: (0,0)-(40,50), center = (20, 25)
	assert.Equal(t, image.Point{X: 20, Y: 25}, grid.CellCenter(0))
	// Cell 2: (80,0)-(120,50), center = (100, 25)
	assert.Equal(t, image.Point{X: 100, Y: 25}, grid.CellCenter(2))
	// Cell 4: (40,50)-(80,100), center = (60, 75)
	assert.Equal(t, image.Point{X: 60, Y: 75}, grid.CellCenter(4))
}

func TestSelectCell_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithDimensions(bounds, 10, 10, 10, 3, 2)
	// Select cell 4 (row1, col1) -> bounds narrow to (40,50)-(80,100)
	center, completed := grid.SelectCell(4)
	assert.Equal(t, image.Point{X: 60, Y: 75}, center, "Center of cell 4")
	assert.False(t, completed, "Should not be completed")
	assert.Equal(t, 1, grid.CurrentDepth())
	assert.Equal(t, image.Rect(40, 50, 80, 100), grid.CurrentBounds())
}

func TestCanDivide_NonSquare(t *testing.T) {
	// 3 cols × 2 rows on 120×100 bounds with minSize=50
	// cellWidth = 120/3 = 40, cellHeight = 100/2 = 50
	// 40 < 50 → cannot divide
	grid := recursivegrid.NewRecursiveGridWithDimensions(
		image.Rect(0, 0, 120, 100), 50, 50, 10, 3, 2,
	)
	assert.False(t, grid.CanDivide(), "Width 40 < minSize 50, should not divide")
	// 2 cols × 3 rows on 100×120 bounds with minSize=50
	// cellWidth = 100/2 = 50, cellHeight = 120/3 = 40
	// 40 < 50 → cannot divide
	grid2 := recursivegrid.NewRecursiveGridWithDimensions(
		image.Rect(0, 0, 100, 120), 50, 50, 10, 2, 3,
	)
	assert.False(t, grid2.CanDivide(), "Height 40 < minSize 50, should not divide")
	// Both dimensions large enough
	grid3 := recursivegrid.NewRecursiveGridWithDimensions(
		image.Rect(0, 0, 300, 200), 50, 50, 10, 3, 2,
	)
	assert.True(t, grid3.CanDivide(), "Width 100 and height 100 both >= 50")
}

func TestGridDimensionAccessors(t *testing.T) {
	grid := recursivegrid.NewRecursiveGridWithDimensions(
		image.Rect(0, 0, 100, 100), 10, 10, 10, 3, 2,
	)
	assert.Equal(t, 3, grid.GridCols())
	assert.Equal(t, 2, grid.GridRows())
}

func TestIsComplete(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGrid(bounds, 25, 25, 10)

	assert.False(t, grid.IsComplete(), "Should not be complete initially")

	// Select until we can't divide anymore
	for grid.CanDivide() {
		grid.SelectCell(recursivegrid.TopLeft)
	}

	assert.True(t, grid.IsComplete(), "Should be complete when CanDivide returns false")
}
