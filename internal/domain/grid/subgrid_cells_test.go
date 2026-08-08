package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
)

// TestSubgridCells pins the rectangles a subgrid divides a cell into. The
// numbers here are worked by hand rather than recomputed from the formula, so
// that a change to the formula has to disagree with them.
func TestSubgridCells(t *testing.T) {
	testCases := []struct {
		name   string
		bounds image.Rectangle
		dims   domain.GridDimensions
		want   []image.Rectangle
	}{
		{
			name:   "a cell that divides evenly divides into equal thirds",
			bounds: image.Rect(0, 0, 300, 150),
			dims:   domain.SubgridDimensions(),
			want: []image.Rectangle{
				image.Rect(0, 0, 100, 50),
				image.Rect(100, 0, 200, 50),
				image.Rect(200, 0, 300, 50),
				image.Rect(0, 50, 100, 100),
				image.Rect(100, 50, 200, 100),
				image.Rect(200, 50, 300, 100),
				image.Rect(0, 100, 100, 150),
				image.Rect(100, 100, 200, 150),
				image.Rect(200, 100, 300, 150),
			},
		},
		{
			// 10 wide over 3 columns rounds each break to the nearest pixel —
			// 10/3 = 3.33 rounds to 3, 20/3 = 6.67 rounds to 7 — so the odd
			// pixel lands in the middle column rather than at one end.
			name:   "a cell that does not divide evenly rounds each break",
			bounds: image.Rect(0, 0, 10, 8),
			dims:   domain.SubgridDimensions(),
			want: []image.Rectangle{
				image.Rect(0, 0, 3, 3),
				image.Rect(3, 0, 7, 3),
				image.Rect(7, 0, 10, 3),
				image.Rect(0, 3, 3, 5),
				image.Rect(3, 3, 7, 5),
				image.Rect(7, 3, 10, 5),
				image.Rect(0, 5, 3, 8),
				image.Rect(3, 5, 7, 8),
				image.Rect(7, 5, 10, 8),
			},
		},
		{
			// The same 10x8 cell, moved to where a grid cell actually sits.
			name:   "a cell away from the origin divides where it is",
			bounds: image.Rect(100, 50, 110, 58),
			dims:   domain.SubgridDimensions(),
			want: []image.Rectangle{
				image.Rect(100, 50, 103, 53),
				image.Rect(103, 50, 107, 53),
				image.Rect(107, 50, 110, 53),
				image.Rect(100, 53, 103, 55),
				image.Rect(103, 53, 107, 55),
				image.Rect(107, 53, 110, 55),
				image.Rect(100, 55, 103, 58),
				image.Rect(103, 55, 107, 58),
				image.Rect(107, 55, 110, 58),
			},
		},
		{
			// Not the shipped subgrid: the manager carries its own row and
			// column count, so the shape has to come from the caller.
			name:   "a subgrid that is not square divides by its own counts",
			bounds: image.Rect(0, 0, 9, 5),
			dims:   domain.GridDimensions{Rows: 2, Cols: 4},
			want: []image.Rectangle{
				image.Rect(0, 0, 2, 3),
				image.Rect(2, 0, 5, 3),
				image.Rect(5, 0, 7, 3),
				image.Rect(7, 0, 9, 3),
				image.Rect(0, 3, 2, 5),
				image.Rect(2, 3, 5, 5),
				image.Rect(5, 3, 7, 5),
				image.Rect(7, 3, 9, 5),
			},
		},
		{
			name:   "a cell narrower than its columns still gives one cell per key",
			bounds: image.Rect(0, 0, 2, 2),
			dims:   domain.SubgridDimensions(),
			want: []image.Rectangle{
				image.Rect(0, 0, 1, 1),
				image.Rect(1, 0, 1, 1),
				image.Rect(1, 0, 2, 1),
				image.Rect(0, 1, 1, 1),
				image.Rect(1, 1, 1, 1),
				image.Rect(1, 1, 2, 1),
				image.Rect(0, 1, 1, 2),
				image.Rect(1, 1, 1, 2),
				image.Rect(1, 1, 2, 2),
			},
		},
		{
			name:   "a subgrid with no rows has no cells",
			bounds: image.Rect(0, 0, 300, 150),
			dims:   domain.GridDimensions{Rows: 0, Cols: domain.SubgridCols},
			want:   nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := grid.SubgridCells(testCase.bounds, testCase.dims)
			if len(got) != len(testCase.want) {
				t.Fatalf("SubgridCells(%v, %+v) returned %d cells, want %d",
					testCase.bounds, testCase.dims, len(got), len(testCase.want))
			}

			for index, want := range testCase.want {
				if got[index] != want {
					t.Errorf("cell %d = %v, want %v", index, got[index], want)
				}
			}
		})
	}
}

// TestSubgridCells_CoversTheCellExactly is the property the worked examples
// above are instances of, checked across the widths a rounded division has
// anything to say about. A subgrid that stopped short of its parent's far edge
// would leave a strip of the cell no key reaches, and one that overshot would
// draw over the cell next door.
func TestSubgridCells_CoversTheCellExactly(t *testing.T) {
	const origin = 37

	for size := 1; size <= 64; size++ {
		bounds := image.Rect(origin, origin, origin+size, origin+size)
		cells := grid.SubgridCells(bounds, domain.SubgridDimensions())

		if len(cells) != domain.SubgridRows*domain.SubgridCols {
			t.Fatalf("a %dx%d cell divided into %d cells, want %d",
				size, size, len(cells), domain.SubgridRows*domain.SubgridCols)
		}

		if first := cells[0]; first.Min != bounds.Min {
			t.Errorf("a %dx%d cell starts its subgrid at %v, want %v",
				size, size, first.Min, bounds.Min)
		}

		if last := cells[len(cells)-1]; last.Max != bounds.Max {
			t.Errorf("a %dx%d cell ends its subgrid at %v, want %v",
				size, size, last.Max, bounds.Max)
		}

		for index := 1; index < len(cells); index++ {
			previous, current := cells[index-1], cells[index]

			if index%domain.SubgridCols != 0 && previous.Max.X != current.Min.X {
				t.Errorf("a %dx%d cell leaves a seam between columns at %d and %d",
					size, size, previous.Max.X, current.Min.X)
			}

			if index%domain.SubgridCols == 0 && previous.Max.Y != current.Min.Y {
				t.Errorf("a %dx%d cell leaves a seam between rows at %d and %d",
					size, size, previous.Max.Y, current.Min.Y)
			}
		}
	}
}

// TestManager_SubgridSelectionLandsInsideTheCellItDrew is the stake: the
// rectangles an overlay draws and the point a keypress moves the cursor to are
// the same arithmetic, so the cursor has to land in the cell the key was
// written on. Four copies of that arithmetic used to agree by coincidence.
func TestManager_SubgridSelectionLandsInsideTheCellItDrew(t *testing.T) {
	const configured = "asdfghjkl"

	drawn := grid.SubgridKeys(configured, domain.SubgridRows*domain.SubgridCols)

	for index, key := range drawn {
		manager, opened := newSubgridKeyManager(t, configured)

		cells := grid.SubgridCells(opened, domain.SubgridDimensions())

		point, selected := manager.HandleInput(string(key))
		if !selected {
			t.Fatalf("subgrid refused %q, which it is drawn with", string(key))
		}

		if !point.In(cells[index]) {
			t.Errorf("key %q draws %v but moves the cursor to %v",
				string(key), cells[index], point)
		}
	}
}

// TestManager_SubgridSelectionLandsInsideTheCellItDrew_NonSquare is the same
// stake on a subgrid whose two counts differ, which is the only shape that can
// tell a row count from a column count. The shipped subgrid is 3x3, so a
// manager that had its two counts the wrong way round would divide the cell
// into exactly the same nine rectangles and this whole file would still pass —
// the mistake would surface only for whoever configured a subgrid that was not
// square, and would look like a rendering bug rather than a swapped pair.
func TestManager_SubgridSelectionLandsInsideTheCellItDrew_NonSquare(t *testing.T) {
	// Four columns over two rows, and eight keys to name the eight cells.
	dims := domain.GridDimensions{Rows: 2, Cols: 4}

	const configured = "asdfghjk"

	drawn := grid.SubgridKeys(configured, dims.CellCount())
	if len(drawn) != dims.CellCount() {
		t.Fatalf("SubgridKeys(%q) drew %d keys, want %d",
			configured, len(drawn), dims.CellCount())
	}

	for index, key := range drawn {
		manager, opened := newSubgridManager(t, dims, configured)

		cells := grid.SubgridCells(opened, dims)

		point, selected := manager.HandleInput(string(key))
		if !selected {
			t.Fatalf("subgrid refused %q, which it is drawn with", string(key))
		}

		if !point.In(cells[index]) {
			t.Errorf("key %q draws %v but moves the cursor to %v",
				string(key), cells[index], point)
		}
	}
}
