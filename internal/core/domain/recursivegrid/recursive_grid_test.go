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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

	cells := grid.Divide()

	// Verify 4 cells
	assert.Len(t, cells, 4, "Should have 4 cells for 2x2 grid")

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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

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
	// - First division creates 25x25 cells (for 2x2 grid)
	// - 25/2 = 12 < 25, so CanDivide returns false after first selection
	bounds := image.Rect(0, 0, 50, 50)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

	// Select top-left - bounds narrow to (0,0)-(25,25), but NOT completed yet.
	// The user gets one more selection at this final depth.
	center, completed := grid.SelectCell(recursivegrid.TopLeft)

	// After selecting top-left, bounds are (0,0)-(25,25), center rounded to nearest pixel
	expectedCenter := image.Point{X: 13, Y: 13}
	assert.Equal(t, expectedCenter, center, "Center should be at (13, 13)")
	assert.False(
		t,
		completed,
		"Should NOT be completed yet — user gets one more selection at final depth",
	)

	// Now at final depth (CanDivide is false), selecting a sub-cell completes
	center2, completed2 := grid.SelectCell(recursivegrid.BottomRight)
	assert.True(t, completed2, "Should be completed after selection at final depth")

	// Cells are distributed contiguously: (0,0)-(13,13), (13,0)-(25,13),
	// (0,13)-(13,25), (13,13)-(25,25). BottomRight cell is (13,13)-(25,25), center rounded.
	expectedCenter2 := image.Point{X: 19, Y: 19}
	assert.Equal(t, expectedCenter2, center2, "Center should be at (19, 19)")
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGridWithLayers(
				testCase.bounds, testCase.minSize, testCase.minSize, testCase.maxDepth, 2, 2, nil,
			)
			result := grid.CanDivide()
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func TestBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

	tl := grid.CellBounds(recursivegrid.TopLeft)
	expected := image.Rect(0, 0, 50, 50)
	assert.Equal(t, expected, tl, "Top-left bounds should be correct")

	br := grid.CellBounds(recursivegrid.BottomRight)
	expected = image.Rect(50, 50, 100, 100)
	assert.Equal(t, expected, br, "Bottom-right bounds should be correct")
}

func TestCellCenter(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

	center := grid.CellCenter(recursivegrid.TopRight)
	expected := image.Point{X: 75, Y: 25}
	assert.Equal(t, expected, center, "Top-right center should be at (75, 25)")
}

func TestDivide_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 10, 3, 2, nil)
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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 10, 2, 3, nil)
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
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 10, 3, 2, nil)
	// Cell 0: (0,0)-(40,50), center = (20, 25)
	assert.Equal(t, image.Point{X: 20, Y: 25}, grid.CellCenter(0))
	// Cell 2: (80,0)-(120,50), center = (100, 25)
	assert.Equal(t, image.Point{X: 100, Y: 25}, grid.CellCenter(2))
	// Cell 4: (40,50)-(80,100), center = (60, 75)
	assert.Equal(t, image.Point{X: 60, Y: 75}, grid.CellCenter(4))
}

func TestSelectCell_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 10, 3, 2, nil)
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
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 120, 100), 50, 50, 10, 3, 2, nil,
	)
	assert.False(t, grid.CanDivide(), "Width 40 < minSize 50, should not divide")
	// 2 cols × 3 rows on 100×120 bounds with minSize=50
	// cellWidth = 100/2 = 50, cellHeight = 120/3 = 40
	// 40 < 50 → cannot divide
	grid2 := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 100, 120), 50, 50, 10, 2, 3, nil,
	)
	assert.False(t, grid2.CanDivide(), "Height 40 < minSize 50, should not divide")
	// Both dimensions large enough
	grid3 := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 300, 200), 50, 50, 10, 3, 2, nil,
	)
	assert.True(t, grid3.CanDivide(), "Width 100 and height 100 both >= 50")
}

func TestGridDimensionAccessors(t *testing.T) {
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 100, 100), 10, 10, 10, 3, 2, nil,
	)
	assert.Equal(t, 3, grid.GridCols())
	assert.Equal(t, 2, grid.GridRows())
}

func TestRemapToNewBounds_PreservesDepthAndHistory(t *testing.T) {
	// Start with a 100×100 grid, select top-left twice to build history.
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 10, 2, 2, nil)
	// Depth 0 → 1: currentBounds narrows to (0,0)-(50,50)
	grid.SelectCell(recursivegrid.TopLeft)
	// Depth 1 → 2: currentBounds narrows to (0,0)-(25,25)
	grid.SelectCell(recursivegrid.TopLeft)
	assert.Equal(t, 2, grid.CurrentDepth(), "Depth should be 2 before remap")
	// Remap to a 200×200 screen (2× scale).
	newBounds := image.Rect(0, 0, 200, 200)
	grid.RemapToNewBounds(newBounds)
	// Depth and history length must be preserved.
	assert.Equal(t, 2, grid.CurrentDepth(), "Depth should still be 2 after remap")
	// currentBounds (0,0)-(25,25) on 100×100 → (0,0)-(50,50) on 200×200
	assert.Equal(t, image.Rect(0, 0, 50, 50), grid.CurrentBounds(),
		"Current bounds should be proportionally remapped")
	// initialBounds should be updated.
	assert.Equal(t, newBounds, grid.InitialBounds(),
		"Initial bounds should be updated to new bounds")
	// Backtrack should restore the remapped parent bounds.
	grid.Backtrack()
	// History[1] was (0,0)-(50,50) on 100×100 → (0,0)-(100,100) on 200×200
	assert.Equal(t, image.Rect(0, 0, 100, 100), grid.CurrentBounds(),
		"Backtracked bounds should be proportionally remapped")
	grid.Backtrack()
	// History[0] was (0,0)-(100,100) on 100×100 → (0,0)-(200,200) on 200×200
	assert.Equal(t, newBounds, grid.CurrentBounds(),
		"Fully backtracked bounds should equal new initial bounds")
}

func TestRemapToNewBounds_NonOriginScreen(t *testing.T) {
	// Simulate a screen that doesn't start at (0,0), e.g., a secondary monitor.
	oldBounds := image.Rect(0, 0, 1000, 500)
	grid := recursivegrid.NewRecursiveGridWithLayers(oldBounds, 10, 10, 10, 2, 2, nil)
	// Select bottom-right: currentBounds → (500,250)-(1000,500)
	grid.SelectCell(recursivegrid.BottomRight)
	// Remap to a new screen with different origin and size.
	newBounds := image.Rect(0, 0, 2000, 1000)
	grid.RemapToNewBounds(newBounds)
	// (500,250)-(1000,500) on 1000×500 → (1000,500)-(2000,1000) on 2000×1000
	assert.Equal(t, image.Rect(1000, 500, 2000, 1000), grid.CurrentBounds())
}

func TestRemapToNewBounds_RoundTripMinimizesDrift(t *testing.T) {
	// Remap 1920→1080→1920 and verify the coordinates return close to original.
	// With rounding, drift should be ≤1px per coordinate; without rounding
	// (truncation) the error can be larger.
	bounds := image.Rect(0, 0, 1920, 1080)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 10, 10, 20, 2, 2, nil)
	// Build some depth so history has non-trivial coordinates.
	grid.SelectCell(recursivegrid.BottomRight) // (960,540)-(1920,1080)
	grid.SelectCell(recursivegrid.TopLeft)     // (960,540)-(1440,810)
	originalBounds := grid.CurrentBounds()
	// Remap to a smaller screen and back.
	grid.RemapToNewBounds(image.Rect(0, 0, 1080, 720))
	grid.RemapToNewBounds(image.Rect(0, 0, 1920, 1080))
	result := grid.CurrentBounds()
	// Allow ≤1px drift per edge due to integer rounding.
	for _, pair := range [][2]int{
		{originalBounds.Min.X, result.Min.X},
		{originalBounds.Min.Y, result.Min.Y},
		{originalBounds.Max.X, result.Max.X},
		{originalBounds.Max.Y, result.Max.Y},
	} {
		diff := pair[0] - pair[1]
		if diff < 0 {
			diff = -diff
		}

		assert.LessOrEqual(t, diff, 1,
			"Round-trip drift should be ≤1px, got original=%d result=%d", pair[0], pair[1])
	}
}

func TestRemapToNewBounds_ZeroOldBounds(t *testing.T) {
	// Edge case: zero-size old bounds should not panic (division by zero guard).
	oldBounds := image.Rect(0, 0, 0, 0)
	grid := recursivegrid.NewRecursiveGridWithLayers(oldBounds, 0, 0, 10, 2, 2, nil)
	newBounds := image.Rect(0, 0, 200, 200)
	grid.RemapToNewBounds(newBounds)
	// With zero old bounds, currentBounds should fall back to newBounds.
	assert.Equal(t, newBounds, grid.CurrentBounds())
}

// verifyContiguousCoverage checks that the cells from Divide() form a perfect
// partition of the given bounds — no gaps, no overlaps, no overflow.
func verifyContiguousCoverage(
	t *testing.T,
	cells []image.Rectangle,
	bounds image.Rectangle,
	cols int,
	rows int,
) {
	t.Helper()

	if len(cells) != cols*rows {
		t.Fatalf("expected %d cells, got %d", cols*rows, len(cells))
	}

	for idx, cell := range cells {
		// No overflow: every cell must be within bounds
		if cell.Min.X < bounds.Min.X {
			t.Errorf("cell %d: Min.X %d < bounds.Min.X %d", idx, cell.Min.X, bounds.Min.X)
		}

		if cell.Min.Y < bounds.Min.Y {
			t.Errorf("cell %d: Min.Y %d < bounds.Min.Y %d", idx, cell.Min.Y, bounds.Min.Y)
		}

		if cell.Max.X > bounds.Max.X {
			t.Errorf("cell %d: Max.X %d > bounds.Max.X %d", idx, cell.Max.X, bounds.Max.X)
		}

		if cell.Max.Y > bounds.Max.Y {
			t.Errorf("cell %d: Max.Y %d > bounds.Max.Y %d", idx, cell.Max.Y, bounds.Max.Y)
		}

		// Every cell must have positive area
		if cell.Dx() <= 0 {
			t.Errorf("cell %d: non-positive width %d", idx, cell.Dx())
		}

		if cell.Dy() <= 0 {
			t.Errorf("cell %d: non-positive height %d", idx, cell.Dy())
		}
	}

	for row := range rows {
		for col := range cols {
			idx := row*cols + col
			cell := cells[idx]

			// First cell in each row must start at bounds.Min.X
			if col == 0 && cell.Min.X != bounds.Min.X {
				t.Errorf(
					"row %d, col 0: Min.X %d != bounds.Min.X %d",
					row, cell.Min.X, bounds.Min.X,
				)
			}

			// Last cell in each row must end at bounds.Max.X
			if col == cols-1 && cell.Max.X != bounds.Max.X {
				t.Errorf(
					"row %d, last col: Max.X %d != bounds.Max.X %d",
					row, cell.Max.X, bounds.Max.X,
				)
			}

			// Horizontal contiguity: adjacent cells must touch exactly
			if col > 0 {
				prev := cells[row*cols+(col-1)]
				if cell.Min.X != prev.Max.X {
					t.Errorf(
						"horizontal gap/overlap row %d between col %d and %d: "+
							"prev.Max.X=%d, curr.Min.X=%d",
						row, col-1, col, prev.Max.X, cell.Min.X,
					)
				}
			}

			// First row cells must start at bounds.Min.Y
			if row == 0 && cell.Min.Y != bounds.Min.Y {
				t.Errorf(
					"row 0, col %d: Min.Y %d != bounds.Min.Y %d",
					col, cell.Min.Y, bounds.Min.Y,
				)
			}

			// Last row cells must end at bounds.Max.Y
			if row == rows-1 && cell.Max.Y != bounds.Max.Y {
				t.Errorf(
					"last row, col %d: Max.Y %d != bounds.Max.Y %d",
					col, cell.Max.Y, bounds.Max.Y,
				)
			}

			// Vertical contiguity: cells in the same column on adjacent rows
			// must touch exactly.
			if row > 0 {
				above := cells[(row-1)*cols+col]
				if cell.Min.Y != above.Max.Y {
					t.Errorf(
						"vertical gap/overlap col %d between row %d and %d: "+
							"above.Max.Y=%d, curr.Min.Y=%d",
						col, row-1, row, above.Max.Y, cell.Min.Y,
					)
				}
			}
		}
	}
}

// verifyCellSizeConsistency checks that cell widths within each row differ by
// at most 1 pixel, and cell heights within each column differ by at most 1 pixel.
func verifyCellSizeConsistency(t *testing.T, cells []image.Rectangle, cols int, rows int) {
	t.Helper()

	for row := range rows {
		minW, maxW := cells[row*cols].Dx(), cells[row*cols].Dx()

		for col := range cols {
			cellWidth := cells[row*cols+col].Dx()
			if cellWidth < minW {
				minW = cellWidth
			}

			if cellWidth > maxW {
				maxW = cellWidth
			}
		}

		if maxW-minW > 1 {
			t.Errorf(
				"row %d: cell width variance too large: min=%d, max=%d (diff=%d, max allowed=1)",
				row, minW, maxW, maxW-minW,
			)
		}
	}

	for col := range cols {
		minH, maxH := cells[col].Dy(), cells[col].Dy()

		for row := range rows {
			cellHeight := cells[row*cols+col].Dy()
			if cellHeight < minH {
				minH = cellHeight
			}

			if cellHeight > maxH {
				maxH = cellHeight
			}
		}

		if maxH-minH > 1 {
			t.Errorf(
				"col %d: cell height variance too large: min=%d, max=%d (diff=%d, max allowed=1)",
				col, minH, maxH, maxH-minH,
			)
		}
	}
}

func TestDivide_ContiguousCoverage(t *testing.T) {
	tests := []struct {
		name     string
		bounds   image.Rectangle
		gridCols int
		gridRows int
	}{
		// 2x2 variations
		{name: "2x2_even_100x100", bounds: image.Rect(0, 0, 100, 100), gridCols: 2, gridRows: 2},
		{name: "2x2_uneven_101x99", bounds: image.Rect(0, 0, 101, 99), gridCols: 2, gridRows: 2},
		{
			name:     "2x2_offset_500x300_700x500",
			bounds:   image.Rect(500, 300, 700, 500),
			gridCols: 2,
			gridRows: 2,
		},

		// 3x3 variations
		{name: "3x3_even_300x300", bounds: image.Rect(0, 0, 300, 300), gridCols: 3, gridRows: 3},
		{name: "3x3_uneven_100x100", bounds: image.Rect(0, 0, 100, 100), gridCols: 3, gridRows: 3},
		{name: "3x3_rem2_101x101", bounds: image.Rect(0, 0, 101, 101), gridCols: 3, gridRows: 3},
		{
			name:     "3x3_prime_1920x1080",
			bounds:   image.Rect(0, 0, 1920, 1080),
			gridCols: 3,
			gridRows: 3,
		},

		// 4x4 variations
		{name: "4x4_even_400x400", bounds: image.Rect(0, 0, 400, 400), gridCols: 4, gridRows: 4},
		{name: "4x4_uneven_101x101", bounds: image.Rect(0, 0, 101, 101), gridCols: 4, gridRows: 4},
		{name: "4x4_rem3_103x103", bounds: image.Rect(0, 0, 103, 103), gridCols: 4, gridRows: 4},
		{
			name:     "4x4_offset_100x200_300x400",
			bounds:   image.Rect(100, 200, 300, 400),
			gridCols: 4,
			gridRows: 4,
		},

		// 5x5 variations
		{name: "5x5_even_500x500", bounds: image.Rect(0, 0, 500, 500), gridCols: 5, gridRows: 5},
		{name: "5x5_uneven_100x100", bounds: image.Rect(0, 0, 100, 100), gridCols: 5, gridRows: 5},
		{name: "5x5_rem4_104x104", bounds: image.Rect(0, 0, 104, 104), gridCols: 5, gridRows: 5},
		{
			name:     "5x5_prime_1920x1080",
			bounds:   image.Rect(0, 0, 1920, 1080),
			gridCols: 5,
			gridRows: 5,
		},

		// Weird edge combos: non-square grids
		{name: "3x2_uneven_101x101", bounds: image.Rect(0, 0, 101, 101), gridCols: 3, gridRows: 2},
		{name: "2x3_uneven_102x103", bounds: image.Rect(0, 0, 102, 103), gridCols: 2, gridRows: 3},
		{name: "4x3_even_400x300", bounds: image.Rect(0, 0, 400, 300), gridCols: 4, gridRows: 3},
		{name: "3x4_uneven_101x99", bounds: image.Rect(0, 0, 101, 99), gridCols: 3, gridRows: 4},
		{name: "5x3_uneven_103x100", bounds: image.Rect(0, 0, 103, 100), gridCols: 5, gridRows: 3},
		{name: "3x5_uneven_100x103", bounds: image.Rect(0, 0, 100, 103), gridCols: 3, gridRows: 5},
		{name: "6x2_uneven_100x50", bounds: image.Rect(0, 0, 100, 50), gridCols: 6, gridRows: 2},
		{name: "2x6_uneven_50x100", bounds: image.Rect(0, 0, 50, 100), gridCols: 2, gridRows: 6},
		{
			name:     "7x3_uneven_1920x1080",
			bounds:   image.Rect(0, 0, 1920, 1080),
			gridCols: 7,
			gridRows: 3,
		},
		{
			name:     "3x7_uneven_1080x1920",
			bounds:   image.Rect(0, 0, 1080, 1920),
			gridCols: 3,
			gridRows: 7,
		},

		// Single row/column
		{
			name:     "3x1_single_row_100x50",
			bounds:   image.Rect(0, 0, 100, 50),
			gridCols: 3,
			gridRows: 1,
		},
		{
			name:     "1x3_single_col_50x100",
			bounds:   image.Rect(0, 0, 50, 100),
			gridCols: 1,
			gridRows: 3,
		},
		{
			name:     "5x1_single_row_101x30",
			bounds:   image.Rect(0, 0, 101, 30),
			gridCols: 5,
			gridRows: 1,
		},
		{
			name:     "1x5_single_col_30x101",
			bounds:   image.Rect(0, 0, 30, 101),
			gridCols: 1,
			gridRows: 5,
		},
		{
			name:     "4x1_single_row_offset_200x100_400x150",
			bounds:   image.Rect(200, 100, 400, 150),
			gridCols: 4,
			gridRows: 1,
		},

		// Large remainder scenarios
		{
			name:     "4x3_large_rem_1919x1079",
			bounds:   image.Rect(0, 0, 1919, 1079),
			gridCols: 4,
			gridRows: 3,
		},
		{name: "6x4_max_rem_100x100", bounds: image.Rect(0, 0, 100, 100), gridCols: 6, gridRows: 4},
		{
			name:     "8x3_large_grid_2000x1000",
			bounds:   image.Rect(0, 0, 2000, 1000),
			gridCols: 8,
			gridRows: 3,
		},

		// Prime-ish combinations
		{
			name:     "7x5_prime_dims_1920x1080",
			bounds:   image.Rect(0, 0, 1920, 1080),
			gridCols: 7,
			gridRows: 5,
		},
		{
			name:     "5x7_prime_swapped_1080x1920",
			bounds:   image.Rect(0, 0, 1080, 1920),
			gridCols: 5,
			gridRows: 7,
		},
		{
			name:     "11x7_large_prime_2000x1500",
			bounds:   image.Rect(0, 0, 2000, 1500),
			gridCols: 11,
			gridRows: 7,
		},

		// Tiny cells (many divisions)
		{name: "9x9_tiny_100x100", bounds: image.Rect(0, 0, 100, 100), gridCols: 9, gridRows: 9},
		{
			name:     "10x10_dense_200x200",
			bounds:   image.Rect(0, 0, 200, 200),
			gridCols: 10,
			gridRows: 10,
		},

		// Oddball combinations
		{
			name:     "4x2_offset_nonzero_origin",
			bounds:   image.Rect(400, 200, 800, 500),
			gridCols: 4,
			gridRows: 2,
		},
		{name: "2x4_tall_offset", bounds: image.Rect(100, 100, 300, 500), gridCols: 2, gridRows: 4},
		{
			name:     "6x6_square_uneven_250x250",
			bounds:   image.Rect(0, 0, 250, 250),
			gridCols: 6,
			gridRows: 6,
		},
		{name: "8x5_mixed_999x777", bounds: image.Rect(0, 0, 999, 777), gridCols: 8, gridRows: 5},
		{
			name:     "3x8_tall_narrow_500x2000",
			bounds:   image.Rect(0, 0, 500, 2000),
			gridCols: 3,
			gridRows: 8,
		},
		{
			name:     "12x4_wide_2000x500",
			bounds:   image.Rect(0, 0, 2000, 500),
			gridCols: 12,
			gridRows: 4,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGridWithLayers(
				testCase.bounds,
				1, 1, 10,
				testCase.gridCols, testCase.gridRows,
				nil,
			)
			cells := grid.Divide()

			verifyContiguousCoverage(
				t, cells,
				testCase.bounds,
				testCase.gridCols, testCase.gridRows,
			)
			verifyCellSizeConsistency(
				t, cells,
				testCase.gridCols, testCase.gridRows,
			)
		})
	}
}

func TestDivide_ContiguousAcrossDepths(t *testing.T) {
	// Regression test: after selecting a cell and recursing, the next level's
	// cells must exactly cover the cell's bounds with no gaps or overflow.
	bounds := image.Rect(0, 0, 800, 600)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 1, 1, 10, 3, 3, nil)

	// Depth 0: verify top-level grid
	layout := grid.LayoutForDepth(grid.CurrentDepth())
	cells0 := grid.Divide()
	verifyContiguousCoverage(t, cells0, bounds, layout.GridCols, layout.GridRows)
	verifyCellSizeConsistency(t, cells0, layout.GridCols, layout.GridRows)

	// Select center cell (index 4 in a 3x3 grid) → depth 1
	// The new currentBounds must exactly equal the selected cell bounds
	centerCell := cells0[4]
	_, completed := grid.SelectCell(4)
	assert.False(t, completed, "Should not complete after first selection")
	assert.Equal(t, centerCell, grid.CurrentBounds(),
		"CurrentBounds after selecting must match the cell from Divide()")

	// Depth 1: verify cells are contiguous within the selected cell
	layout1 := grid.LayoutForDepth(grid.CurrentDepth())
	cells1 := grid.Divide()
	verifyContiguousCoverage(t, cells1, grid.CurrentBounds(), layout1.GridCols, layout1.GridRows)
	verifyCellSizeConsistency(t, cells1, layout1.GridCols, layout1.GridRows)

	// Select top-left cell (index 0) → depth 2
	topLeftCell := cells1[0]
	_, completed = grid.SelectCell(0)
	assert.False(t, completed, "Should not complete after second selection")
	assert.Equal(t, topLeftCell, grid.CurrentBounds(),
		"CurrentBounds after second selection must match the cell from Divide()")

	// Depth 2: verify cells are contiguous within the twice-narrowed bounds
	layout2 := grid.LayoutForDepth(grid.CurrentDepth())
	cells2 := grid.Divide()
	verifyContiguousCoverage(t, cells2, grid.CurrentBounds(), layout2.GridCols, layout2.GridRows)
	verifyCellSizeConsistency(t, cells2, layout2.GridCols, layout2.GridRows)
}

func TestDivide_ContiguousAcrossDepths_UnevenBounds(t *testing.T) {
	// Real-world scenario: 1080p display with 3x3 grid.
	// 1920/3 = 640, 1080/3 = 360 — divides evenly for the top level.
	// Sub-recursions will produce uneven divisions that must still be contiguous.
	bounds := image.Rect(0, 0, 1920, 1080)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 1, 1, 10, 3, 3, nil)

	// Depth 0: all cells should be exactly 640x360
	layout := grid.LayoutForDepth(grid.CurrentDepth())
	cells0 := grid.Divide()
	verifyContiguousCoverage(t, cells0, bounds, layout.GridCols, layout.GridRows)
	verifyCellSizeConsistency(t, cells0, layout.GridCols, layout.GridRows)

	// Select bottom-right cell (index 8) → (1280,720)-(1920,1080)
	_, completed := grid.SelectCell(8)
	assert.False(t, completed)

	// Depth 1: bounds (1280,720)-(1920,1080), 640x360, still divides evenly
	layout1 := grid.LayoutForDepth(grid.CurrentDepth())
	cells1 := grid.Divide()
	verifyContiguousCoverage(t, cells1, grid.CurrentBounds(), layout1.GridCols, layout1.GridRows)
	verifyCellSizeConsistency(t, cells1, layout1.GridCols, layout1.GridRows)

	// Select top-left cell (index 0) → (1280,720)-(1493,840) approx
	// At 3x3: 640/3 = 213rem1, 360/3 = 120rem0
	_, completed = grid.SelectCell(0)
	assert.False(t, completed)

	// Depth 2: bounds should be divisible into 3x3 contiguous cells
	layout2 := grid.LayoutForDepth(grid.CurrentDepth())
	cells2 := grid.Divide()
	verifyContiguousCoverage(t, cells2, grid.CurrentBounds(), layout2.GridCols, layout2.GridRows)
	verifyCellSizeConsistency(t, cells2, layout2.GridCols, layout2.GridRows)

	// Select one more cell to depth 3 with heavily uneven dimensions
	_, completed = grid.SelectCell(4) // center cell at depth 2
	assert.False(t, completed)

	// Depth 3: must still be contiguous
	layout3 := grid.LayoutForDepth(grid.CurrentDepth())
	cells3 := grid.Divide()
	verifyContiguousCoverage(t, cells3, grid.CurrentBounds(), layout3.GridCols, layout3.GridRows)
	verifyCellSizeConsistency(t, cells3, layout3.GridCols, layout3.GridRows)
}

func TestDivide_ContiguousAcrossDepths_NonSquareGrids(t *testing.T) {
	// Test with 2x4 grid across depths, including per-depth layout overrides.
	bounds := image.Rect(0, 0, 1000, 700)
	depthLayouts := map[int]recursivegrid.DepthLayout{
		0: {GridCols: 2, GridRows: 4},
		1: {GridCols: 3, GridRows: 2},
		2: {GridCols: 5, GridRows: 1},
	}
	grid := recursivegrid.NewRecursiveGridWithLayers(
		bounds, 1, 1, 10,
		3, 3, // defaults (overridden at each depth)
		depthLayouts,
	)

	// Depth 0: 2x4
	layout := grid.LayoutForDepth(0)
	cells := grid.Divide()
	verifyContiguousCoverage(t, cells, bounds, layout.GridCols, layout.GridRows)
	verifyCellSizeConsistency(t, cells, layout.GridCols, layout.GridRows)

	// Select cell 0 → depth 1
	selected := cells[0]
	_, completed := grid.SelectCell(0)
	assert.False(t, completed)
	assert.Equal(t, selected, grid.CurrentBounds())

	// Depth 1: 3x2 within selected cell
	layout1 := grid.LayoutForDepth(1)
	cells1 := grid.Divide()
	verifyContiguousCoverage(t, cells1, grid.CurrentBounds(), layout1.GridCols, layout1.GridRows)
	verifyCellSizeConsistency(t, cells1, layout1.GridCols, layout1.GridRows)

	// Select cell 0 → depth 2
	selected2 := cells1[0]
	_, completed = grid.SelectCell(0)
	assert.False(t, completed)
	assert.Equal(t, selected2, grid.CurrentBounds())

	// Depth 2: 5x1 (single row) within twice-narrowed bounds
	layout2 := grid.LayoutForDepth(2)
	cells2 := grid.Divide()
	verifyContiguousCoverage(t, cells2, grid.CurrentBounds(), layout2.GridCols, layout2.GridRows)
	verifyCellSizeConsistency(t, cells2, layout2.GridCols, layout2.GridRows)
}

func TestIsComplete(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(bounds, 25, 25, 10, 2, 2, nil)

	assert.False(t, grid.IsComplete(), "Should not be complete initially")

	// Select until we can't divide anymore
	for grid.CanDivide() {
		grid.SelectCell(recursivegrid.TopLeft)
	}

	assert.True(t, grid.IsComplete(), "Should be complete when CanDivide returns false")
}
