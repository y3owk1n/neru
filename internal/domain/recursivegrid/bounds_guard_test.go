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
	return recursivegrid.NewRecursiveGrid(image.Rect(0, 0, 800, 600), 10, 10, 5)
}

// Cell indices reach these methods straight from a keypress: the key-to-cell
// map is built from user-configurable key strings, and a stale or mismatched
// layout override can yield an index outside the current depth's cell list.
// Every accessor therefore guards the index and falls back to the current
// bounds rather than indexing out of range.
//
// The guards were untested, which is the dangerous combination: an inverted
// comparison in one of them turns a mistyped key into a panic that takes the
// daemon down, and the tests would not have noticed.
//
// TestRecursiveGrid_CellCenter_RejectsOutOfRangeCells pins the documented
// fallback: an index outside the cell list yields the center of the current
// bounds, and never panics.
func TestRecursiveGrid_CellCenter_RejectsOutOfRangeCells(t *testing.T) {
	grid := newGuardGrid()
	cellCount := len(grid.Divide())

	if cellCount == 0 {
		t.Fatal("grid divided into no cells")
	}

	want := grid.CurrentCenter()

	for _, cell := range outOfRangeCells(cellCount) {
		if got := grid.CellCenter(cell); got != want {
			t.Errorf("CellCenter(%d) = %v, want the current center %v", cell, got, want)
		}
	}

	// Every in-range index must instead return a point inside that cell — the
	// guard must not be so eager that it swallows valid selections.
	cells := grid.Divide()
	for idx, bounds := range cells {
		center := grid.CellCenter(recursivegrid.Cell(idx))
		if !center.In(bounds) {
			t.Errorf("CellCenter(%d) = %v, outside that cell's bounds %v", idx, center, bounds)
		}
	}
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

		// An invalid selection must not descend: the bounds and depth have to
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

// TestRecursiveGrid_SelectCell_ValidCellDescends is the counterpart: the guard
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
// rounding helper on the path that actually produces negative coordinates: on
// macOS a display positioned left of or above the primary has a negative
// origin, so a monitor change remaps the active region into negative space.
// Rounding that truncates toward zero instead of to nearest drifts the region
// by a pixel per remap, in opposite directions either side of the origin.
func TestRecursiveGrid_RemapToNewBounds_HandlesNegativeOrigins(t *testing.T) {
	tests := []struct {
		name     string
		from, to image.Rectangle
	}{
		{
			"primary to a display left of it",
			image.Rect(0, 0, 1920, 1080),
			image.Rect(-1920, 0, 0, 1080),
		},
		{
			"primary to a display above it",
			image.Rect(0, 0, 1920, 1080),
			image.Rect(0, -1080, 1920, 0),
		},
		{
			"negative origin back to primary",
			image.Rect(-1920, -1080, 0, 0),
			image.Rect(0, 0, 1920, 1080),
		},
		{"odd sizes across the origin", image.Rect(-1001, -733, 0, 0), image.Rect(0, 0, 1367, 769)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := recursivegrid.NewRecursiveGrid(testCase.from, 10, 10, 5)

			// Descend twice so the remap has a non-trivial region to carry.
			grid.SelectCell(recursivegrid.TopRight)
			grid.SelectCell(recursivegrid.BottomLeft)

			depthBefore := grid.CurrentDepth()

			grid.RemapToNewBounds(testCase.to)

			// The remapped region must land inside the new display, or the
			// recursive grid is drawn off-screen after a monitor change.
			if got := grid.CurrentBounds(); !got.In(testCase.to) {
				t.Errorf("after remap CurrentBounds() = %v, outside the new bounds %v",
					got, testCase.to)
			}

			if got := grid.CurrentBounds(); got.Empty() {
				t.Errorf("after remap CurrentBounds() = %v, which is empty", got)
			}

			// Remapping repositions; it must not change how deep the user is.
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
