package recursivegrid

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	domainrecursivegrid "github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// TestStyle_PreviewsNextDepth pins the per-draw gate on the sub-key preview:
// the option is on, the next depth has keys, and it has a shape to divide a cell
// into.
//
// The zero next layout is the case that matters. The mode leaves it zero when
// the region can no longer be divided, and the deepest level of a recursive grid
// must draw no preview at all — a mini-grid of keys nothing can select is worse
// than a bare cell. The Cairo and GDI backends both ask this, so the cases run
// in every job rather than only where a particular backend is built.
func TestStyle_PreviewsNextDepth(t *testing.T) {
	enabled := NewStyle(StyleOptions{SubKeyPreview: true})
	disabled := NewStyle(StyleOptions{SubKeyPreview: false})

	tests := []struct {
		name  string
		style Style
		keys  []rune
		dims  domain.GridDimensions
		want  bool
	}{
		{
			name:  "enabled with keys and a shape",
			style: enabled,
			keys:  []rune("RTYFGHVBN"),
			dims:  domain.GridDimensions{Rows: 3, Cols: 3},
			want:  true,
		},
		{
			name:  "option off",
			style: disabled,
			keys:  []rune("RTYFGHVBN"),
			dims:  domain.GridDimensions{Rows: 3, Cols: 3},
			want:  false,
		},
		{
			name:  "the deepest level, where the next layout is zero",
			style: enabled,
			keys:  nil,
			dims:  domain.GridDimensions{},
			want:  false,
		},
		{
			name:  "keys without a shape",
			style: enabled,
			keys:  []rune("RTYFGHVBN"),
			dims:  domain.GridDimensions{},
			want:  false,
		},
		{
			name:  "a shape without keys",
			style: enabled,
			keys:  nil,
			dims:  domain.GridDimensions{Rows: 3, Cols: 3},
			want:  false,
		},
		{
			name:  "a shape with no columns",
			style: enabled,
			keys:  []rune("RT"),
			dims:  domain.GridDimensions{Rows: 2, Cols: 0},
			want:  false,
		},
		{
			name:  "a shape with no rows",
			style: enabled,
			keys:  []rune("RT"),
			dims:  domain.GridDimensions{Rows: 0, Cols: 2},
			want:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.style.PreviewsNextDepth(len(testCase.keys), testCase.dims)
			if got != testCase.want {
				t.Errorf(
					"PreviewsNextDepth(%d, %+v) = %v, want %v",
					len(testCase.keys), testCase.dims, got, testCase.want,
				)
			}
		})
	}
}

// TestStyle_ShowSubKeyPreviewIn pins the sub-key-preview autohide threshold every
// backend draws by: sub_key_preview_autohide_multiplier x the preview font size,
// compared against a *sub-cell* on both axes, with a non-positive multiplier
// meaning "always show".
//
// The sub-cell is the point. A cell that clears the threshold whole while the
// sub-cells it divides into do not is exactly the shape the GDI backend used to
// keep drawing in, and the case below named for it is what holds all three
// backends to one answer now (#1297).
func TestStyle_ShowSubKeyPreviewIn(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		cell       image.Rectangle
		nextDims   domain.GridDimensions
		fontSize   int
		multiplier float64
		want       bool
	}{
		{
			name:       "the option off hides whatever the size",
			enabled:    false,
			cell:       image.Rect(0, 0, 900, 900),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2,
			want:       false,
		},
		{
			name:       "zero multiplier always shows",
			enabled:    true,
			cell:       image.Rect(0, 0, 4, 4),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   100,
			multiplier: 0,
			want:       true,
		},
		{
			name:       "negative multiplier always shows",
			enabled:    true,
			cell:       image.Rect(0, 0, 4, 4),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   100,
			multiplier: -2,
			want:       true,
		},
		{
			name:       "sub-cells clear the threshold on both axes",
			enabled:    true,
			cell:       image.Rect(0, 0, 90, 90),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 30x30
			want:       true,
		},
		{
			name:       "sub-cells exactly on the threshold show",
			enabled:    true,
			cell:       image.Rect(0, 0, 60, 60),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 20x20
			want:       true,
		},
		{
			name:       "a whole cell over the threshold whose sub-cells are under it",
			enabled:    true,
			cell:       image.Rect(0, 0, 40, 40),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 13.33x13.33
			want:       false,
		},
		{
			name:       "narrow sub-cells hide even when tall enough",
			enabled:    true,
			cell:       image.Rect(0, 0, 57, 90),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 19x30
			want:       false,
		},
		{
			name:       "short sub-cells hide even when wide enough",
			enabled:    true,
			cell:       image.Rect(0, 0, 90, 57),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 30x19
			want:       false,
		},
		{
			name:       "rows and columns are not interchangeable",
			enabled:    true,
			cell:       image.Rect(0, 0, 60, 40),
			nextDims:   domain.GridDimensions{Rows: 2, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 20x20
			want:       true,
		},
		{
			name:       "an undivided axis is measured whole",
			enabled:    true,
			cell:       image.Rect(0, 0, 20, 60),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 1},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 20x20
			want:       true,
		},
		{
			name:       "offset cell is measured by its size, not its position",
			enabled:    true,
			cell:       image.Rect(500, 700, 590, 790),
			nextDims:   domain.GridDimensions{Rows: 3, Cols: 3},
			fontSize:   10,
			multiplier: 2, // threshold 20, sub-cells 30x30
			want:       true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			style := NewStyle(StyleOptions{
				SubKeyPreview:                   testCase.enabled,
				SubKeyPreviewFontSize:           testCase.fontSize,
				SubKeyPreviewAutohideMultiplier: testCase.multiplier,
			})

			got := style.ShowSubKeyPreviewIn(testCase.cell, testCase.nextDims)
			if got != testCase.want {
				t.Errorf(
					"ShowSubKeyPreviewIn(%v, %+v) = %v, want %v",
					testCase.cell, testCase.nextDims, got, testCase.want,
				)
			}
		})
	}
}

// TestStyle_SubKeyPreviewCells_PlacesEachKeyOnTheCellItSelects is the whole point
// of the mini-grid: a previewed key has to sit where the cell it selects will be
// drawn, or it tells the user the wrong thing.
//
// So the expectation is not a hand-written rectangle list but the subdivision the
// next depth is actually drawn with, which is what makes this a pin rather than a
// restatement.
func TestStyle_SubKeyPreviewCells_PlacesEachKeyOnTheCellItSelects(t *testing.T) {
	style := NewStyle(StyleOptions{SubKeyPreview: true})
	cell := image.Rect(100, 200, 190, 260)
	nextDims := domain.GridDimensions{Rows: 2, Cols: 3}
	nextKeys := []rune("RTYFGH")

	want := domainrecursivegrid.ComputeGridCells(cell, nextDims)

	preview := style.SubKeyPreviewCells(cell, nextKeys, nextDims)
	if len(preview) != len(want) {
		t.Fatalf("SubKeyPreviewCells() returned %d sub-cells, want %d", len(preview), len(want))
	}

	for idx, subCell := range preview {
		if subCell.Bounds != want[idx] {
			t.Errorf(
				"sub-cell %d bounds = %v, want %v",
				idx, subCell.Bounds, want[idx],
			)
		}

		if subCell.Label != string(nextKeys[idx]) {
			t.Errorf("sub-cell %d label = %q, want %q", idx, subCell.Label, string(nextKeys[idx]))
		}
	}
}

// TestStyle_SubKeyPreviewCells_LeavesTheCenterOfAnOddDivisionBlank pins the one
// sub-cell that carries no key. The cell's own label is drawn dead center, so a
// preview key there would be drawn under it.
func TestStyle_SubKeyPreviewCells_LeavesTheCenterOfAnOddDivisionBlank(t *testing.T) {
	style := NewStyle(StyleOptions{SubKeyPreview: true})
	cell := image.Rect(0, 0, 90, 90)
	nextDims := domain.GridDimensions{Rows: 3, Cols: 3}
	nextKeys := []rune("RTYFGHVBN")

	preview := style.SubKeyPreviewCells(cell, nextKeys, nextDims)
	if len(preview) != len(nextKeys)-1 {
		t.Fatalf(
			"SubKeyPreviewCells() returned %d sub-cells, want %d",
			len(preview),
			len(nextKeys)-1,
		)
	}

	center := domainrecursivegrid.ComputeGridCells(cell, nextDims)[4]

	for _, subCell := range preview {
		if subCell.Bounds == center {
			t.Errorf("the center sub-cell %v was labeled %q, want it left blank",
				center, subCell.Label)
		}

		if subCell.Label == "G" {
			t.Errorf("the center key %q was drawn, want it left out", subCell.Label)
		}
	}
}

// TestStyle_SubKeyPreviewCells_EvenDivisionsLabelEverySubCell is the other half
// of the rule above: an even dimension has no center sub-cell for the cell's own
// label to collide with, so every key is drawn.
func TestStyle_SubKeyPreviewCells_EvenDivisionsLabelEverySubCell(t *testing.T) {
	style := NewStyle(StyleOptions{SubKeyPreview: true})

	tests := []struct {
		name string
		dims domain.GridDimensions
		keys string
	}{
		{name: "even by even", dims: domain.GridDimensions{Rows: 2, Cols: 2}, keys: "RTFG"},
		{name: "odd by even", dims: domain.GridDimensions{Rows: 3, Cols: 2}, keys: "RTFGVB"},
		{name: "even by odd", dims: domain.GridDimensions{Rows: 2, Cols: 3}, keys: "RTYFGH"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			preview := style.SubKeyPreviewCells(
				image.Rect(0, 0, 120, 120), []rune(testCase.keys), testCase.dims,
			)
			if len(preview) != len(testCase.keys) {
				t.Fatalf(
					"SubKeyPreviewCells() returned %d sub-cells, want %d",
					len(preview), len(testCase.keys),
				)
			}
		})
	}
}

// TestStyle_SubKeyPreviewCells_LabelCharOverridesEveryKey pins what
// sub_key_preview_label_char asks for: one character on every sub-cell instead of
// the key that selects it, for a user who wants the shape of the next level
// without its keys.
func TestStyle_SubKeyPreviewCells_LabelCharOverridesEveryKey(t *testing.T) {
	style := NewStyle(StyleOptions{SubKeyPreview: true, SubKeyPreviewLabelChar: "."})

	preview := style.SubKeyPreviewCells(
		image.Rect(0, 0, 90, 90),
		[]rune("RTYFGHVBN"),
		domain.GridDimensions{Rows: 3, Cols: 3},
	)

	for idx, subCell := range preview {
		if subCell.Label != "." {
			t.Errorf("sub-cell %d label = %q, want %q", idx, subCell.Label, ".")
		}
	}
}

// TestStyle_SubKeyPreviewCells_DrawsNothingWithoutANextDepth holds the layout to
// the same gate PreviewsNextDepth answers, so a backend that forgets to ask it
// draws nothing rather than dividing a cell by zero.
func TestStyle_SubKeyPreviewCells_DrawsNothingWithoutANextDepth(t *testing.T) {
	tests := []struct {
		name  string
		style Style
		keys  []rune
		dims  domain.GridDimensions
	}{
		{
			name:  "the option off",
			style: NewStyle(StyleOptions{SubKeyPreview: false}),
			keys:  []rune("RTYFGHVBN"),
			dims:  domain.GridDimensions{Rows: 3, Cols: 3},
		},
		{
			name:  "the deepest level, where the next layout is zero",
			style: NewStyle(StyleOptions{SubKeyPreview: true}),
			keys:  nil,
			dims:  domain.GridDimensions{},
		},
		{
			name:  "a shape with no columns",
			style: NewStyle(StyleOptions{SubKeyPreview: true}),
			keys:  []rune("RT"),
			dims:  domain.GridDimensions{Rows: 2, Cols: 0},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			preview := testCase.style.SubKeyPreviewCells(
				image.Rect(0, 0, 90, 90), testCase.keys, testCase.dims,
			)
			if len(preview) != 0 {
				t.Errorf("SubKeyPreviewCells() returned %d sub-cells, want none", len(preview))
			}
		})
	}
}

// TestStyle_SubKeyPreviewCells_FewerKeysThanSubCellsLeavesTheRestBlank pins the
// degenerate configuration rather than leaving it to panic: a key mapping shorter
// than the division labels what it can and stops.
func TestStyle_SubKeyPreviewCells_FewerKeysThanSubCellsLeavesTheRestBlank(t *testing.T) {
	style := NewStyle(StyleOptions{SubKeyPreview: true})

	preview := style.SubKeyPreviewCells(
		image.Rect(0, 0, 120, 120),
		[]rune("RT"),
		domain.GridDimensions{Rows: 2, Cols: 2},
	)

	if len(preview) != 2 {
		t.Fatalf("SubKeyPreviewCells() returned %d sub-cells, want 2", len(preview))
	}
}
