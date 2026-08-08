package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// TestComputeGridCells pins the rectangles a recursive-grid level divides its
// bounds into. The numbers are worked by hand rather than recomputed from the
// formula, so a change to the formula has to disagree with them.
//
// Every case here is deliberately non-square, and the two dimensions are
// deliberately different in the remainder they leave. A division that read the
// row count as the column count would have to produce these same rectangles to
// pass, and it cannot: a 5-column, 2-row grid of a 13x7 rectangle and a
// 2-column, 5-row one share neither their cell count per row nor their edges.
func TestComputeGridCells(t *testing.T) {
	testCases := []struct {
		name   string
		bounds image.Rectangle
		dims   domain.GridDimensions
		want   []image.Rectangle
	}{
		{
			// 13 across 5 columns is 2 each with 3 spare, which go to the
			// first three columns: 3, 3, 3, 2, 2. 7 down 2 rows is 3 each
			// with 1 spare, which goes to the first row: 4, 3.
			name:   "spare pixels go to the first cells of each axis",
			bounds: image.Rect(0, 0, 13, 7),
			dims:   domain.GridDimensions{Rows: 2, Cols: 5},
			want: []image.Rectangle{
				image.Rect(0, 0, 3, 4),
				image.Rect(3, 0, 6, 4),
				image.Rect(6, 0, 9, 4),
				image.Rect(9, 0, 11, 4),
				image.Rect(11, 0, 13, 4),
				image.Rect(0, 4, 3, 7),
				image.Rect(3, 4, 6, 7),
				image.Rect(6, 4, 9, 7),
				image.Rect(9, 4, 11, 7),
				image.Rect(11, 4, 13, 7),
			},
		},
		{
			// The transpose of the case above, on the same bounds: 13 across
			// 2 columns is 6 each with 1 spare (7, 6), and 7 down 5 rows is
			// 1 each with 2 spare (2, 2, 1, 1, 1). Nothing about the answer
			// resembles the one above.
			name:   "swapping the two counts divides the same bounds differently",
			bounds: image.Rect(0, 0, 13, 7),
			dims:   domain.GridDimensions{Rows: 5, Cols: 2},
			want: []image.Rectangle{
				image.Rect(0, 0, 7, 2),
				image.Rect(7, 0, 13, 2),
				image.Rect(0, 2, 7, 4),
				image.Rect(7, 2, 13, 4),
				image.Rect(0, 4, 7, 5),
				image.Rect(7, 4, 13, 5),
				image.Rect(0, 5, 7, 6),
				image.Rect(7, 5, 13, 6),
				image.Rect(0, 6, 7, 7),
				image.Rect(7, 6, 13, 7),
			},
		},
		{
			// The same non-square division, moved to where a monitor that is
			// not the primary one actually sits.
			name:   "bounds away from the origin divide where they are",
			bounds: image.Rect(100, 50, 113, 57),
			dims:   domain.GridDimensions{Rows: 2, Cols: 5},
			want: []image.Rectangle{
				image.Rect(100, 50, 103, 54),
				image.Rect(103, 50, 106, 54),
				image.Rect(106, 50, 109, 54),
				image.Rect(109, 50, 111, 54),
				image.Rect(111, 50, 113, 54),
				image.Rect(100, 54, 103, 57),
				image.Rect(103, 54, 106, 57),
				image.Rect(106, 54, 109, 57),
				image.Rect(109, 54, 111, 57),
				image.Rect(111, 54, 113, 57),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := recursivegrid.ComputeGridCells(testCase.bounds, testCase.dims)
			if len(got) != len(testCase.want) {
				t.Fatalf("ComputeGridCells(%v, %+v) returned %d cells, want %d",
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
