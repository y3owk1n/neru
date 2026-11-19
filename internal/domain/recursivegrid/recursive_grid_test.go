package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

func TestCanDivide(t *testing.T) {
	square := domain.GridDimensions{Rows: 2, Cols: 2}

	tests := []struct {
		name     string
		bounds   image.Rectangle
		dims     domain.GridDimensions
		minSize  int
		maxDepth int
		expected bool
	}{
		{
			name:     "Can divide 100x100 with min 25",
			bounds:   image.Rect(0, 0, 100, 100),
			dims:     square,
			minSize:  25,
			maxDepth: 10,
			expected: true,
		},
		{
			name:     "Cannot divide 40x40 with min 25",
			bounds:   image.Rect(0, 0, 40, 40),
			dims:     square,
			minSize:  25,
			maxDepth: 10,
			expected: false, // 40/2 = 20 < 25
		},
		{
			name:     "Can divide 50x50 with min 25",
			bounds:   image.Rect(0, 0, 50, 50),
			dims:     square,
			minSize:  25,
			maxDepth: 10,
			expected: true, // 50/2 = 25 >= 25
		},
		{
			name:     "Cannot divide when max depth reached",
			bounds:   image.Rect(0, 0, 1000, 1000),
			dims:     square,
			minSize:  1,
			maxDepth: 0,
			expected: false, // depth starts at 0, max is 0
		},
		{
			name:     "Cannot divide 3 cols x 2 rows when the width falls short",
			bounds:   image.Rect(0, 0, 120, 100),
			dims:     domain.GridDimensions{Rows: 2, Cols: 3},
			minSize:  50,
			maxDepth: 10,
			expected: false, // 120/3 = 40 < 50, 100/2 = 50
		},
		{
			name:     "Cannot divide 2 cols x 3 rows when the height falls short",
			bounds:   image.Rect(0, 0, 100, 120),
			dims:     domain.GridDimensions{Rows: 3, Cols: 2},
			minSize:  50,
			maxDepth: 10,
			expected: false, // 100/2 = 50, 120/3 = 40 < 50
		},
		{
			name:     "Can divide 3 cols x 2 rows when both axes fit",
			bounds:   image.Rect(0, 0, 300, 200),
			dims:     domain.GridDimensions{Rows: 2, Cols: 3},
			minSize:  50,
			maxDepth: 10,
			expected: true, // 300/3 = 100, 200/2 = 100
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGridWithLayers(
				testCase.bounds,
				testCase.minSize,
				testCase.minSize,
				testCase.maxDepth,
				testCase.dims,
				nil,
			)
			result := grid.CanDivide()
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func TestBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(
		bounds,
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil,
	)

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

// TestDivide_NonSquare3x2 divides a grid whose two counts differ, both ways
// round. Either shape alone fails if Divide transposes the counts.
func TestDivide_NonSquare3x2(t *testing.T) {
	testCases := []struct {
		name   string
		bounds image.Rectangle
		dims   domain.GridDimensions
		want   []image.Rectangle
	}{
		{
			// Cell widths 120/3 = 40, heights 100/2 = 50.
			name:   "3 cols x 2 rows",
			bounds: image.Rect(0, 0, 120, 100),
			dims:   domain.GridDimensions{Rows: 2, Cols: 3},
			want: []image.Rectangle{
				image.Rect(0, 0, 40, 50),
				image.Rect(40, 0, 80, 50),
				image.Rect(80, 0, 120, 50),
				image.Rect(
					0,
					50,
					40,
					100,
				),
				image.Rect(40, 50, 80, 100),
				image.Rect(80, 50, 120, 100),
			},
		},
		{
			// Cell widths 100/2 = 50, heights 120/3 = 40.
			name:   "2 cols x 3 rows",
			bounds: image.Rect(0, 0, 100, 120),
			dims:   domain.GridDimensions{Rows: 3, Cols: 2},
			want: []image.Rectangle{
				image.Rect(0, 0, 50, 40), image.Rect(50, 0, 100, 40),
				image.Rect(0, 40, 50, 80), image.Rect(50, 40, 100, 80),
				image.Rect(0, 80, 50, 120), image.Rect(50, 80, 100, 120),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGridWithLayers(
				testCase.bounds, 10, 10, 10, testCase.dims, nil,
			)
			assert.Equal(t, testCase.want, grid.Divide(), "cells in row-major order")
		})
	}
}

func TestSelectCell_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(
		bounds,
		10,
		10,
		10,
		domain.GridDimensions{Rows: 2, Cols: 3},
		nil,
	)
	// Select cell 4 (row1, col1) -> bounds narrow to (40,50)-(80,100)
	center, completed := grid.SelectCell(4)
	assert.Equal(t, image.Point{X: 60, Y: 75}, center, "Center of cell 4")
	assert.False(t, completed, "Should not be completed")
	assert.Equal(t, 1, grid.CurrentDepth())
	assert.Equal(t, image.Rect(40, 50, 80, 100), grid.CurrentBounds())
}

func TestRemapToNewBounds_PreservesDepthAndHistory(t *testing.T) {
	// Start with a 100x100 grid, select top-left twice to build history.
	bounds := image.Rect(0, 0, 100, 100)
	grid := recursivegrid.NewRecursiveGridWithLayers(
		bounds,
		10,
		10,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil,
	)
	// Depth 0 -> 1: currentBounds narrows to (0,0)-(50,50)
	grid.SelectCell(recursivegrid.TopLeft)
	// Depth 1 -> 2: currentBounds narrows to (0,0)-(25,25)
	grid.SelectCell(recursivegrid.TopLeft)
	assert.Equal(t, 2, grid.CurrentDepth(), "Depth should be 2 before remap")
	// Remap to a 200x200 screen (2x scale).
	newBounds := image.Rect(0, 0, 200, 200)
	grid.RemapToNewBounds(newBounds)
	// Depth and history length must be preserved.
	assert.Equal(t, 2, grid.CurrentDepth(), "Depth should still be 2 after remap")
	// currentBounds (0,0)-(25,25) on 100x100 -> (0,0)-(50,50) on 200x200
	assert.Equal(t, image.Rect(0, 0, 50, 50), grid.CurrentBounds(),
		"Current bounds should be proportionally remapped")
	// initialBounds should be updated.
	assert.Equal(t, newBounds, grid.InitialBounds(),
		"Initial bounds should be updated to new bounds")
	// Backtrack should restore the remapped parent bounds.
	grid.Backtrack()
	// History[1] was (0,0)-(50,50) on 100x100 -> (0,0)-(100,100) on 200x200
	assert.Equal(t, image.Rect(0, 0, 100, 100), grid.CurrentBounds(),
		"Backtracked bounds should be proportionally remapped")
	grid.Backtrack()
	// History[0] was (0,0)-(100,100) on 100x100 -> (0,0)-(200,200) on 200x200
	assert.Equal(t, newBounds, grid.CurrentBounds(),
		"Fully backtracked bounds should equal new initial bounds")
}

func TestRemapToNewBounds_RoundTripMinimizesDrift(t *testing.T) {
	// Remap 1920x1080->1366x768->1920x1080 and verify the coordinates return
	// close to the original. 1366/1920 does not divide these edges evenly, so
	// each remap has a remainder to round. With rounding, drift stays <=1px per
	// coordinate. Truncation would carry the left edge from 240 to 238.
	bounds := image.Rect(0, 0, 1920, 1080)
	grid := recursivegrid.NewRecursiveGridWithLayers(
		bounds,
		10,
		10,
		20,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil,
	)
	// Build some depth so history has non-trivial coordinates.
	grid.SelectCell(recursivegrid.TopLeft)  // (0,0)-(960,540)
	grid.SelectCell(recursivegrid.TopLeft)  // (0,0)-(480,270)
	grid.SelectCell(recursivegrid.TopRight) // (240,0)-(480,135)
	originalBounds := grid.CurrentBounds()
	// Remap to a smaller screen and back.
	grid.RemapToNewBounds(image.Rect(0, 0, 1366, 768))
	grid.RemapToNewBounds(image.Rect(0, 0, 1920, 1080))
	result := grid.CurrentBounds()
	// Allow <=1px drift per edge due to integer rounding.
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
	grid := recursivegrid.NewRecursiveGridWithLayers(
		oldBounds,
		0,
		0,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil,
	)
	newBounds := image.Rect(0, 0, 200, 200)
	grid.RemapToNewBounds(newBounds)
	// With zero old bounds, currentBounds should fall back to newBounds.
	assert.Equal(t, newBounds, grid.CurrentBounds())
}

// verifyContiguousCoverage checks that the cells from Divide() form a perfect
// partition of the given bounds, with no gaps, no overlaps and no overflow.
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
				domain.GridDimensions{Rows: testCase.gridRows, Cols: testCase.gridCols},
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
