package grid

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain"
)

// roundingFactor is what a positive quotient is offset by before it is
// truncated, which rounds it to the nearest whole pixel. It is written once
// here because a break rounded one way and drawn the other is the whole
// problem SubgridCells exists to remove.
const roundingFactor = 0.5

// SubgridCells is the set of rectangles a subgrid divides one cell into, in the
// order its keys name them: left to right, then top to bottom, so that the
// cell at index i is the cell SubgridKeys names at index i.
//
// It exists because those rectangles decide two different things and have to
// decide them the same way. The manager computes them to answer where the
// cursor goes when a subgrid key is pressed; every overlay backend computed
// them again to answer where the cell is drawn. Four copies, agreeing by
// coincidence — and a copy that drifted would draw a cell in one place and
// click in another, which is the one failure a person cannot see coming and no
// test was watching for.
//
// The breaks are rounded rather than truncated so that the columns of a cell
// that does not divide evenly differ by at most a pixel instead of collecting
// the whole remainder in the last one, and the last break is the cell's own far
// edge rather than the rounded value, so the subgrid covers the cell exactly:
// no seam a click can fall into and no cell drawn a pixel past its parent.
//
// The dimensions are a parameter rather than the shipped 3x3
// (domain.SubgridDimensions) because the manager already carries its own, and a
// function that baked the constant in would leave it computing its answer
// somewhere else the moment that stopped being true. They arrive together as a
// domain.GridDimensions rather than as a row count and a column count because
// recursivegrid.ComputeGridCells divides a rectangle too, and the two used to
// take their counts in opposite orders — a transposed call was invisible while
// every grid was square (#1294).
//
// This division is deliberately not recursivegrid.ComputeGridCells, whose doc
// comment says how the two differ and why merging them is not on the table.
//
// A subgrid with no rows or no columns has no cells, which is what a caller
// that has not been configured yet gets instead of a panic.
func SubgridCells(bounds image.Rectangle, dims domain.GridDimensions) []image.Rectangle {
	rows, cols := dims.Rows, dims.Cols
	if rows < 1 || cols < 1 {
		return nil
	}

	xBreaks := make([]int, cols+1)
	yBreaks := make([]int, rows+1)

	for index := range xBreaks {
		xBreaks[index] = bounds.Min.X + roundedBreak(index, bounds.Dx(), cols)
	}

	for index := range yBreaks {
		yBreaks[index] = bounds.Min.Y + roundedBreak(index, bounds.Dy(), rows)
	}

	xBreaks[cols] = bounds.Max.X
	yBreaks[rows] = bounds.Max.Y

	cells := make([]image.Rectangle, 0, dims.CellCount())

	for row := range rows {
		for col := range cols {
			cells = append(cells, image.Rect(
				xBreaks[col], yBreaks[row],
				xBreaks[col+1], yBreaks[row+1],
			))
		}
	}

	return cells
}

// roundedBreak is the offset of the index'th of divisions evenly spaced breaks
// across length, rounded to the nearest pixel.
func roundedBreak(index, length, divisions int) int {
	return int(float64(index)*float64(length)/float64(divisions) + roundingFactor)
}
