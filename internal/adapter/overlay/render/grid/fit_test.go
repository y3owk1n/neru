package grid_test

import (
	"image"
	"testing"
	"unicode/utf8"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// runeWidthPerSize is how wide the fake measurer makes a rune, close to what a
// real sans-serif capital takes.
const runeWidthPerSize = 0.6

// fixedMeasurer measures every rune alike, so a fit is easy to work out by
// hand. A label of n runes fills 0.9 of a cell w wide at 0.9w / 0.6n.
type fixedMeasurer struct{}

func (fixedMeasurer) Measure(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
	return ports.TextMetrics{
		Width:  float64(utf8.RuneCountInString(text)) * size * runeWidthPerSize,
		Height: size,
	}, nil
}

// fitTheme is a fixed light theme. The fit reads no color.
type fitTheme struct{}

func (fitTheme) IsDarkMode() bool { return false }

func styleAt(t *testing.T, fontSize int) gridcomponent.Style {
	t.Helper()

	ports.SetTextMeasurer(fixedMeasurer{})
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	cfg := config.DefaultConfig().Grid
	cfg.UI.FontSize = fontSize

	return gridcomponent.BuildStyle(cfg, fitTheme{})
}

func smallestCell(grid *domainGrid.Grid) (image.Rectangle, int) {
	cells := grid.AllCells()
	last := cells[len(cells)-1]

	return last.Bounds(), utf8.RuneCountInString(last.Coordinate())
}

func TestStyle_LabelFontSizeFor(t *testing.T) {
	grid := domainGrid.NewGrid("abcdefghij", image.Rect(0, 0, 1920, 1080), zap.NewNop())
	cell, runes := smallestCell(grid)

	t.Run("the default font size is drawn as configured", func(t *testing.T) {
		style := styleAt(t, config.DefaultGridFontSize)

		if got := style.LabelFontSizeFor(1, grid); got != config.DefaultGridFontSize {
			t.Fatalf("LabelFontSizeFor() = %v in a %v cell, want the configured %d",
				got, cell.Size(), config.DefaultGridFontSize)
		}
	})

	t.Run("a font too large for the cells shrinks until the label fits", func(t *testing.T) {
		style := styleAt(t, 200)

		got := style.LabelFontSizeFor(1, grid)

		widthFit := float64(cell.Dx()) * 0.9 / (float64(runes) * runeWidthPerSize)
		heightFit := float64(cell.Dy()) * 0.9 / 0.8

		want := float64(int(min(widthFit, heightFit)))
		if got != want {
			t.Fatalf("LabelFontSizeFor() = %v for %d runes in a %v cell, want %v",
				got, runes, cell.Size(), want)
		}
	})

	t.Run("a dense display is fitted in font units", func(t *testing.T) {
		style := styleAt(t, 200)

		if at1, at2 := style.LabelFontSizeFor(
			1,
			grid,
		), style.LabelFontSizeFor(
			2,
			grid,
		); at2 >= at1 {
			t.Fatalf("LabelFontSizeFor() = %v at 2x and %v at 1x for the same device-pixel cells, "+
				"want the 2x label smaller in font units", at2, at1)
		}
	})
}

func TestStyle_LabelFontSizeFor_NeverHidesALabel(t *testing.T) {
	// Typing the label is the mode, so a grid label is drawn even where it
	// cannot fit, at the smallest size rather than at none.
	style := styleAt(t, 40)
	grid := domainGrid.NewGrid("ab", image.Rect(0, 0, 8, 8), zap.NewNop())

	if got := style.LabelFontSizeFor(1, grid); got != 4 {
		t.Fatalf("LabelFontSizeFor() = %v, want the 4 point floor", got)
	}
}

func TestStyle_SubgridLabelFontSizeIn(t *testing.T) {
	tests := []struct {
		name     string
		fontSize int
		factor   float64
		cells    []image.Rectangle
		want     float64
	}{
		{
			name:     "roomy sub-cells draw the scaled size",
			fontSize: 20,
			factor:   0.7,
			cells:    []image.Rectangle{image.Rect(0, 0, 40, 40)},
			want:     14,
		},
		{
			name:     "the smallest sub-cell decides",
			fontSize: 20,
			factor:   1,
			cells:    []image.Rectangle{image.Rect(0, 0, 40, 40), image.Rect(40, 0, 50, 8)},
			want:     9,
		},
		{
			// The default macOS subgrid, which must draw as configured.
			name:     "a 10 point capital fits a 10 point sub-cell as configured",
			fontSize: 10,
			factor:   1,
			cells:    []image.Rectangle{image.Rect(0, 0, 10, 10)},
			want:     10,
		},
		{
			name:     "no sub-cells keep the scaled size",
			fontSize: 20,
			factor:   0.5,
			want:     10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			style := styleAt(t, test.fontSize)

			if got := style.SubgridLabelFontSizeIn(1, test.factor, test.cells); got != test.want {
				t.Fatalf("SubgridLabelFontSizeIn() = %v, want %v", got, test.want)
			}
		})
	}
}
