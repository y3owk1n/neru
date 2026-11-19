package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// outOfRangeCells returns indices that must always be rejected for a grid whose
// current depth has cellCount cells.
func outOfRangeCells(cellCount int) []recursivegrid.Cell {
	return []recursivegrid.Cell{
		recursivegrid.Cell(-1),
		recursivegrid.Cell(-100),
		recursivegrid.Cell(cellCount),
		recursivegrid.Cell(cellCount + 1),
		recursivegrid.Cell(cellCount * 10),
	}
}

func newGuardGrid() *recursivegrid.RecursiveGrid {
	return recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 800, 600), 10, 10, 5, recursivegrid.DefaultDimensions(), nil,
	)
}

// TestRecursiveGrid_CellBounds_RejectsOutOfRangeCells covers the rendering
// path's guard. A bad index here would draw the highlight over uninitialized
// memory-derived coordinates rather than the current region.
func TestRecursiveGrid_CellBounds_RejectsOutOfRangeCells(t *testing.T) {
	grid := newGuardGrid()
	cellCount := len(grid.Divide())

	want := grid.CurrentBounds()

	for _, cell := range outOfRangeCells(cellCount) {
		if got := grid.CellBounds(cell); got != want {
			t.Errorf("CellBounds(%d) = %v, want the current bounds %v", cell, got, want)
		}
	}

	for idx, bounds := range grid.Divide() {
		if got := grid.CellBounds(recursivegrid.Cell(idx)); got != bounds {
			t.Errorf("CellBounds(%d) = %v, want %v", idx, got, bounds)
		}
	}
}

// TestRecursiveGrid_SelectCell_RejectsOutOfRangeCells covers the selection
// guard, which additionally reports completion so the caller exits the mode
// rather than waiting for more input it will never be able to use.
func TestRecursiveGrid_SelectCell_RejectsOutOfRangeCells(t *testing.T) {
	for _, cell := range outOfRangeCells(len(newGuardGrid().Divide())) {
		grid := newGuardGrid()
		wantPoint := grid.CurrentCenter()
		wantBounds := grid.CurrentBounds()
		wantDepth := grid.CurrentDepth()

		got, complete := grid.SelectCell(cell)

		if got != wantPoint {
			t.Errorf("SelectCell(%d) point = %v, want the current center %v", cell, got, wantPoint)
		}

		if !complete {
			t.Errorf("SelectCell(%d) complete = false, want true", cell)
		}

		// An invalid selection must not descend. The bounds and depth have to
		// be exactly where they were, or a later backtrack unwinds to the
		// wrong ancestor.
		if grid.CurrentBounds() != wantBounds {
			t.Errorf("SelectCell(%d) changed bounds to %v, want %v unchanged",
				cell, grid.CurrentBounds(), wantBounds)
		}

		if grid.CurrentDepth() != wantDepth {
			t.Errorf("SelectCell(%d) changed depth to %d, want %d unchanged",
				cell, grid.CurrentDepth(), wantDepth)
		}
	}
}

// TestRecursiveGrid_SelectCell_ValidCellDescends is the counterpart. The guard
// must not reject the boundary indices 0 and cellCount-1, which an off-by-one
// in either comparison would.
func TestRecursiveGrid_SelectCell_ValidCellDescends(t *testing.T) {
	cellCount := len(newGuardGrid().Divide())

	for _, idx := range []int{0, cellCount / 2, cellCount - 1} {
		grid := newGuardGrid()
		cells := grid.Divide()
		wantBounds := cells[idx]

		got, _ := grid.SelectCell(recursivegrid.Cell(idx))

		if grid.CurrentBounds() != wantBounds {
			t.Errorf("SelectCell(%d) narrowed to %v, want the cell's bounds %v",
				idx, grid.CurrentBounds(), wantBounds)
		}

		if grid.CurrentDepth() != 1 {
			t.Errorf("SelectCell(%d) left depth at %d, want 1", idx, grid.CurrentDepth())
		}

		if !got.In(wantBounds) {
			t.Errorf("SelectCell(%d) point %v is outside the selected cell %v",
				idx, got, wantBounds)
		}
	}
}

// TestRecursiveGrid_RemapToNewBounds_HandlesNegativeOrigins exercises the
// rounding helper on the path that actually produces negative coordinates. On
// macOS a display positioned left of or above the primary has a negative
// origin, so a monitor change remaps the active region into negative space.
// Rounding that truncates instead of rounding to nearest drifts the region by a
// pixel per remap, so each case pins the exact rectangle the region lands on.
func TestRecursiveGrid_RemapToNewBounds_HandlesNegativeOrigins(t *testing.T) {
	tests := []struct {
		name     string
		from, to image.Rectangle
		want     image.Rectangle
	}{
		{
			// The region (1067,0)-(1280,120) moves 1920 to the left.
			"primary to a display left of it",
			image.Rect(0, 0, 1920, 1080),
			image.Rect(-1920, 0, 0, 1080),
			image.Rect(-853, 0, -640, 120),
		},
		{
			"primary to a display above it",
			image.Rect(0, 0, 1920, 1080),
			image.Rect(0, -1080, 1920, 0),
			image.Rect(1067, -1080, 1280, -960),
		},
		{
			"negative origin back to primary",
			image.Rect(-1920, -1080, 0, 0),
			image.Rect(0, 0, 1920, 1080),
			image.Rect(1067, 0, 1280, 120),
		},
		{
			// The region (-444,-733)-(-333,-651) sits 557 into a 1001-wide
			// display. 557*1367/1001 = 760.6, which rounds to 761.
			"odd sizes across the origin",
			image.Rect(-1001, -733, 0, 0),
			image.Rect(0, 0, 1367, 769),
			image.Rect(761, 0, 912, 86),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGridWithLayers(
				testCase.from, 10, 10, 5, recursivegrid.DefaultDimensions(), nil,
			)

			// Descend twice so the remap has a non-trivial region to carry.
			grid.SelectCell(recursivegrid.TopRight)
			grid.SelectCell(recursivegrid.BottomLeft)

			depthBefore := grid.CurrentDepth()

			grid.RemapToNewBounds(testCase.to)

			if got := grid.CurrentBounds(); got != testCase.want {
				t.Errorf("after remap CurrentBounds() = %v, want %v", got, testCase.want)
			}

			// Remapping repositions, and it must not change how deep the user is.
			if got := grid.CurrentDepth(); got != depthBefore {
				t.Errorf("after remap CurrentDepth() = %d, want %d unchanged", got, depthBefore)
			}

			// The center must stay inside the region it belongs to.
			if center := grid.CurrentCenter(); !center.In(grid.CurrentBounds()) {
				t.Errorf("after remap CurrentCenter() = %v, outside CurrentBounds() %v",
					center, grid.CurrentBounds())
			}
		})
	}
}
