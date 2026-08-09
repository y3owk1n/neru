package recursivegrid_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// TestUsableDimensions pins the rule that decides whether a configured
// recursive-grid shape can be navigated at all, and what it is replaced with
// when it cannot. It was written twice — once in the manager and once in the
// macOS draw — and the copies had already drifted (#1345), so what this covers
// is the whole rule rather than either site's reading of it.
//
// The expected shapes are literals rather than DefaultDimensions() so that a
// change to the default constants has to disagree with them.
func TestUsableDimensions(t *testing.T) {
	testCases := []struct {
		name     string
		dims     domain.GridDimensions
		want     domain.GridDimensions
		wantUsed bool
	}{
		{
			name:     "square grid is usable as given",
			dims:     domain.GridDimensions{Rows: 3, Cols: 3},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: true,
		},
		{
			name:     "non-square grid is usable as given",
			dims:     domain.GridDimensions{Rows: 2, Cols: 5},
			want:     domain.GridDimensions{Rows: 2, Cols: 5},
			wantUsed: true,
		},
		{
			// Two cells is the smallest shape that can narrow anything, and
			// it is usable in either orientation.
			name:     "a single row of two cells is usable",
			dims:     domain.GridDimensions{Rows: 1, Cols: 2},
			want:     domain.GridDimensions{Rows: 1, Cols: 2},
			wantUsed: true,
		},
		{
			name:     "a single column of two cells is usable",
			dims:     domain.GridDimensions{Rows: 2, Cols: 1},
			want:     domain.GridDimensions{Rows: 2, Cols: 1},
			wantUsed: true,
		},
		{
			// 1x1 has the right shape and cannot subdivide: selecting its one
			// cell narrows nothing, so the grid never bottoms out.
			name:     "1x1 is degenerate and falls back",
			dims:     domain.GridDimensions{Rows: 1, Cols: 1},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			name:     "zero columns falls back",
			dims:     domain.GridDimensions{Rows: 3, Cols: 0},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			name:     "zero rows falls back",
			dims:     domain.GridDimensions{Rows: 0, Cols: 3},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			name:     "the zero value falls back",
			dims:     domain.GridDimensions{},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			name:     "negative columns falls back",
			dims:     domain.GridDimensions{Rows: 3, Cols: -2},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			name:     "negative rows falls back",
			dims:     domain.GridDimensions{Rows: -2, Cols: 3},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			// Both negative multiply to a positive cell count, so the cell
			// count alone cannot reject this pair.
			name:     "both dimensions negative falls back",
			dims:     domain.GridDimensions{Rows: -3, Cols: -3},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
		{
			// The whole pair is replaced, not just the offending half: a grid
			// of 7 columns and no rows would otherwise become 7x3, a shape
			// nobody configured and no key mapping matches.
			name:     "one bad dimension replaces both",
			dims:     domain.GridDimensions{Rows: 0, Cols: 7},
			want:     domain.GridDimensions{Rows: 3, Cols: 3},
			wantUsed: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, asGiven := recursivegrid.UsableDimensions(testCase.dims)
			if got != testCase.want {
				t.Errorf("UsableDimensions(%+v) = %+v, want %+v",
					testCase.dims, got, testCase.want)
			}

			if asGiven != testCase.wantUsed {
				t.Errorf("UsableDimensions(%+v) reported as-given %t, want %t",
					testCase.dims, asGiven, testCase.wantUsed)
			}
		})
	}
}

// TestUsableDimensions_FallbackIsTheDefaultShape holds the replacement to the
// shape the rest of the package falls back to, so the two cannot drift apart
// even though the case table above spells the numbers out.
func TestUsableDimensions_FallbackIsTheDefaultShape(t *testing.T) {
	got, asGiven := recursivegrid.UsableDimensions(domain.GridDimensions{Rows: 1, Cols: 1})
	if asGiven {
		t.Fatal("UsableDimensions() reported a 1x1 grid as given")
	}

	if want := recursivegrid.DefaultDimensions(); got != want {
		t.Errorf("UsableDimensions() fell back to %+v, want %+v", got, want)
	}
}
