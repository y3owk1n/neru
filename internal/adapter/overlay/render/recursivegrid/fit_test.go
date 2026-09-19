package recursivegrid

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

// squareMeasurer measures every rune as wide as the font is tall.
type squareMeasurer struct{}

func (squareMeasurer) Measure(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
	return ports.TextMetrics{Width: float64(len([]rune(text))) * size, Height: size}, nil
}

// cellsOf is the nine cells of a 3x3 draw, each side pixels square.
func cellsOf(side int) []image.Rectangle {
	cells := make([]image.Rectangle, 9)
	for idx := range cells {
		cells[idx] = image.Rect(idx*side, 0, (idx+1)*side, side)
	}

	return cells
}

// TestStyle_FitDraw_ShrinksTheLabelWithTheCells is #1691: font_size 20 drew at
// the first depth and nothing by the third. It now shrinks with the cells and
// hides only under min_font_size.
func TestStyle_FitDraw_ShrinksTheLabelWithTheCells(t *testing.T) {
	style := NewStyle(StyleOptions{
		FontSize:                20,
		MinFontSize:             6,
		LabelAutohideMultiplier: 1.5,
	})

	tests := []struct {
		name     string
		cellSide int
		wantSize float64
		wantShow bool
	}{
		{
			name:     "first depth keeps the configured size",
			cellSide: 480,
			wantSize: 20,
			wantShow: true,
		},
		{
			name:     "a cell at the old threshold keeps it too",
			cellSide: 30,
			wantSize: 20,
			wantShow: true,
		},
		{name: "third depth shrinks instead of hiding", cellSide: 18, wantSize: 12, wantShow: true},
		{name: "the floor itself still shows", cellSide: 9, wantSize: 6, wantShow: true},
		{name: "under the floor hides", cellSide: 8, wantShow: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fitted := style.FitDraw(1, cellsOf(test.cellSide), domain.GridDimensions{})

			if fitted.ShowLabel != test.wantShow {
				t.Fatalf(
					"ShowLabel = %v, want %v (size %v)",
					fitted.ShowLabel,
					test.wantShow,
					fitted.LabelSize,
				)
			}

			if test.wantShow && fitted.LabelSize != test.wantSize {
				t.Fatalf("LabelSize = %v, want %v", fitted.LabelSize, test.wantSize)
			}
		})
	}
}

func TestStyle_FitDraw_EveryLabelSharesTheSizeTheSmallestCellFits(t *testing.T) {
	style := NewStyle(StyleOptions{
		FontSize:                20,
		MinFontSize:             6,
		LabelAutohideMultiplier: 1.5,
	})

	// Remainder pixels make the leading cells of a draw one pixel larger.
	cells := []image.Rectangle{image.Rect(0, 0, 19, 19), image.Rect(19, 0, 37, 18)}

	fitted := style.FitDraw(1, cells, domain.GridDimensions{})
	if !fitted.ShowLabel || fitted.LabelSize != 12 {
		t.Fatalf("FitDraw() = %+v, want every label at 12", fitted)
	}
}

func TestStyle_FitDraw_DenseDisplayFitsInFontUnits(t *testing.T) {
	// The rule this replaces compared a logical font size with a device-pixel
	// cell, so at 2x it kept labels the drawn font had outgrown.
	style := NewStyle(StyleOptions{
		FontSize:                20,
		MinFontSize:             6,
		LabelAutohideMultiplier: 1.5,
	})

	fitted := style.FitDraw(2, cellsOf(36), domain.GridDimensions{})
	if !fitted.ShowLabel || fitted.LabelSize != 12 {
		t.Fatalf("FitDraw() at 2x = %+v, want the 18 unit cell's 12", fitted)
	}
}

func TestStyle_FitTransition_HoldsTheSizeBothEndsFit(t *testing.T) {
	style := NewStyle(StyleOptions{
		FontSize:                20,
		MinFontSize:             6,
		LabelAutohideMultiplier: 1.5,
	})

	large, small := cellsOf(160), cellsOf(18)

	for name, ends := range map[string][2][]image.Rectangle{
		"zooming in":  {large, small},
		"backing out": {small, large},
	} {
		t.Run(name, func(t *testing.T) {
			held := style.FitTransition(1, ends[0], ends[1], domain.GridDimensions{})
			if !held.ShowLabel || held.LabelSize != 12 {
				t.Fatalf("FitTransition() = %+v, want 12 held throughout", held)
			}
		})
	}

	t.Run("an end under the floor hides the label until it settles", func(t *testing.T) {
		held := style.FitTransition(1, cellsOf(5), large, domain.GridDimensions{})
		if held.ShowLabel {
			t.Fatalf("FitTransition() = %+v, want the label hidden", held)
		}

		if settled := style.FitDraw(1, large, domain.GridDimensions{}); !settled.ShowLabel {
			t.Fatalf("FitDraw() = %+v, want the label back once settled", settled)
		}
	})
}

func TestStyle_FitDraw_PreviewFitsItsSubCell(t *testing.T) {
	style := NewStyle(StyleOptions{
		FontSize:                        20,
		MinFontSize:                     6,
		LabelAutohideMultiplier:         1.5,
		SubKeyPreview:                   true,
		SubKeyPreviewFontSize:           8,
		SubKeyPreviewAutohideMultiplier: 1.5,
	})
	nextDims := domain.GridDimensions{Rows: 3, Cols: 3}

	tests := []struct {
		name     string
		cellSide int
		wantSize float64
		wantShow bool
	}{
		{
			name:     "roomy sub-cells keep the configured size",
			cellSide: 90,
			wantSize: 8,
			wantShow: true,
		},
		{name: "tight sub-cells shrink the preview", cellSide: 27, wantSize: 6, wantShow: true},
		{name: "sub-cells under the floor hide it", cellSide: 24, wantShow: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fitted := style.FitDraw(1, cellsOf(test.cellSide), nextDims)

			if fitted.ShowPreview != test.wantShow {
				t.Fatalf(
					"ShowPreview = %v, want %v (size %v)",
					fitted.ShowPreview,
					test.wantShow,
					fitted.PreviewSize,
				)
			}

			if test.wantShow && fitted.PreviewSize != test.wantSize {
				t.Fatalf("PreviewSize = %v, want %v", fitted.PreviewSize, test.wantSize)
			}
		})
	}

	t.Run("a draw with no next depth has no preview to fit", func(t *testing.T) {
		if fitted := style.FitDraw(1, cellsOf(90), domain.GridDimensions{}); fitted.ShowPreview {
			t.Fatalf("FitDraw() = %+v, want no preview", fitted)
		}
	})
}

func TestBuildStyle_MeasuresTheLabelItWillDraw(t *testing.T) {
	ports.SetTextMeasurer(squareMeasurer{})
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.FontSize = 20
	cfg.UI.LabelAutohideMultiplier = 0

	t.Run("a key is fitted by its measured size once the multiplier is off", func(t *testing.T) {
		style := BuildStyle(cfg, &mockThemeProvider{})

		// One rune as wide as it is tall fills 0.9 of a 20 pixel cell at 18.
		size, show := style.LabelFontSizeIn(1, image.Rect(0, 0, 20, 20))
		if !show || size != 18 {
			t.Fatalf("LabelFontSizeIn() = %v, %v, want 18, true", size, show)
		}
	})

	t.Run("label_char is what gets measured when it replaces the keys", func(t *testing.T) {
		wide := cfg
		wide.UI.LabelChar = "•••"

		style := BuildStyle(wide, &mockThemeProvider{})

		size, show := style.LabelFontSizeIn(1, image.Rect(0, 0, 40, 40))
		if !show || size != 12 {
			t.Fatalf("LabelFontSizeIn() = %v, %v, want the three runes' 12, true", size, show)
		}
	})

	t.Run("a label plate's padding comes out of the cell", func(t *testing.T) {
		plated := cfg
		plated.UI.LabelBackground = true
		plated.UI.LabelBackgroundPaddingX = 3
		plated.UI.LabelBackgroundPaddingY = 3
		plated.UI.LabelBackgroundBorderWidth = 1

		style := BuildStyle(plated, &mockThemeProvider{})

		// 0.9 x 20 less 2 x (3 padding + 1 border) leaves 10.
		size, show := style.LabelFontSizeIn(1, image.Rect(0, 0, 20, 20))
		if !show || size != 10 {
			t.Fatalf("LabelFontSizeIn() = %v, %v, want 10, true", size, show)
		}
	})
}
