package badge_test

import (
	"errors"
	"image"
	"math"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/ports"
)

// errNoTextLayer is what the failing measurer fails with.
var errNoTextLayer = errors.New("no text layer")

// squareMeasurer measures every rune as wide as the font is tall, so a fit is
// easy to work out by hand.
type squareMeasurer struct{ err error }

func (m squareMeasurer) Measure(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
	if m.err != nil {
		return ports.TextMetrics{}, m.err
	}

	return ports.TextMetrics{Width: float64(len([]rune(text))) * size, Height: size}, nil
}

func installMeasurer(t *testing.T, measurer ports.TextMeasurer) {
	t.Helper()

	ports.SetTextMeasurer(measurer)
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })
}

func square(side int) image.Rectangle { return image.Rect(0, 0, side, side) }

func TestFontFit_SizeIn(t *testing.T) {
	// One rune, as wide as it is tall, so the measured cap is 0.9 of the box.
	unit := badge.LabelMetrics{WidthPerSize: 1, HeightPerSize: 1}

	tests := []struct {
		name     string
		fit      badge.FontFit
		box      image.Rectangle
		scale    float64
		wantSize float64
		wantShow bool
	}{
		{
			name:     "a label that fits keeps the configured size",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      square(200),
			scale:    1,
			wantSize: 20,
			wantShow: true,
		},
		{
			name:     "a label is never scaled up",
			fit:      badge.FontFit{Requested: 10, Metrics: unit},
			box:      square(4000),
			scale:    1,
			wantSize: 10,
			wantShow: true,
		},
		{
			name:     "the multiplier shrinks the font instead of hiding the label",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      square(18),
			scale:    1,
			wantSize: 12,
			wantShow: true,
		},
		{
			name:     "the short side decides",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      image.Rect(0, 0, 300, 15),
			scale:    1,
			wantSize: 10,
			wantShow: true,
		},
		{
			name:     "a wide label is capped by its measured width",
			fit:      badge.FontFit{Requested: 20, Metrics: unit.Repeated(3)},
			box:      square(40),
			scale:    1,
			wantSize: 12,
			wantShow: true,
		},
		{
			name: "a label plate's padding comes out of the box",
			fit: badge.FontFit{
				Requested: 20, Metrics: unit, PadX: 5, PadY: 5,
			},
			box:      square(20),
			scale:    1,
			wantSize: 8,
			wantShow: true,
		},
		{
			name:     "below the floor the label hides",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      square(8),
			scale:    1,
			wantShow: false,
		},
		{
			name:     "a fit between the floor and the next pixel shows at the floor",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      image.Rect(0, 0, 12, 12),
			scale:    1.25,
			wantSize: 6,
			wantShow: true,
		},
		{
			name:     "a floor at the configured size never shrinks: fits",
			fit:      badge.FontFit{Requested: 20, Floor: 20, Multiplier: 1.5, Metrics: unit},
			box:      square(30),
			scale:    1,
			wantSize: 20,
			wantShow: true,
		},
		{
			name:     "a floor at the configured size never shrinks: hides",
			fit:      badge.FontFit{Requested: 20, Floor: 20, Multiplier: 1.5, Metrics: unit},
			box:      square(29),
			scale:    1,
			wantShow: false,
		},
		{
			name:     "a floor above the configured size is the configured size",
			fit:      badge.FontFit{Requested: 10, Floor: 40, Metrics: unit},
			box:      square(100),
			scale:    1,
			wantSize: 10,
			wantShow: true,
		},
		{
			name:     "multiplier zero leaves the measured fit in charge",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Metrics: unit},
			box:      square(20),
			scale:    1,
			wantSize: 18,
			wantShow: true,
		},
		{
			// 60 device pixels at 2x is a 30 unit cell, which gets the same answer as
			// the 30 pixel cell at 1x, which the old rule got wrong.
			name:     "a dense display fits in font units, not device pixels",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      square(36),
			scale:    2,
			wantSize: 12,
			wantShow: true,
		},
		{
			name:     "the size lands on a whole device pixel",
			fit:      badge.FontFit{Requested: 20, Floor: 2, Multiplier: 1.5, Metrics: unit},
			box:      square(20),
			scale:    1.5,
			wantSize: 13.0 / 1.5,
			wantShow: true,
		},
		{
			name:     "an empty box shows nothing",
			fit:      badge.FontFit{Requested: 20, Metrics: unit},
			box:      image.Rectangle{},
			scale:    1,
			wantShow: false,
		},
		{
			name:     "a font size that is not positive shows nothing",
			fit:      badge.FontFit{Requested: 0, Metrics: unit},
			box:      square(100),
			scale:    1,
			wantShow: false,
		},
		{
			name:     "a scale that is not positive reads as 1",
			fit:      badge.FontFit{Requested: 20, Floor: 6, Multiplier: 1.5, Metrics: unit},
			box:      square(18),
			scale:    0,
			wantSize: 12,
			wantShow: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size, show := test.fit.SizeIn(test.box, test.scale)

			if show != test.wantShow {
				t.Fatalf("SizeIn() show = %v, want %v (size %v)", show, test.wantShow, size)
			}

			if show && math.Abs(size-test.wantSize) > 1e-9 {
				t.Fatalf("SizeIn() size = %v, want %v", size, test.wantSize)
			}
		})
	}
}

func TestFontFit_SizeIn_FloorAtRequestedAnswersAsTheOldAutohideRule(t *testing.T) {
	// min_font_size = font_size is how somebody keeps "draw it at my size or
	// not at all". For a one-rune label it has to hide exactly where the rule
	// it replaces hid, which is a cell under multiplier x font size on either side.
	const (
		fontSize   = 12.0
		multiplier = 1.5
	)

	fit := badge.FontFit{
		Requested:  fontSize,
		Floor:      fontSize,
		Multiplier: multiplier,
		Metrics:    badge.LabelMetrics{WidthPerSize: 0.7, HeightPerSize: 1.2},
	}

	for width := 1; width <= 40; width++ {
		for height := 1; height <= 40; height++ {
			threshold := fontSize * multiplier
			want := float64(width) >= threshold && float64(height) >= threshold

			_, got := fit.SizeIn(image.Rect(0, 0, width, height), 1)
			if got != want {
				t.Fatalf("%dx%d cell: show = %v, the old rule answers %v", width, height, got, want)
			}
		}
	}
}

func TestFontFit_SizeAcross(t *testing.T) {
	fit := badge.FontFit{
		Requested:  20,
		Floor:      6,
		Multiplier: 1.5,
		Metrics:    badge.LabelMetrics{WidthPerSize: 1, HeightPerSize: 1},
	}

	t.Run("the smallest box decides", func(t *testing.T) {
		size, show := fit.SizeAcross(1, square(300), square(18), square(90))
		if !show || size != 12 {
			t.Fatalf("SizeAcross() = %v, %v, want 12, true", size, show)
		}
	})

	t.Run("one box under the floor hides the label for all of them", func(t *testing.T) {
		_, show := fit.SizeAcross(1, square(300), square(5))
		if show {
			t.Fatal("SizeAcross() shows a label one of its boxes cannot hold")
		}
	})

	t.Run("no boxes shows nothing", func(t *testing.T) {
		_, show := fit.SizeAcross(1)
		if show {
			t.Fatal("SizeAcross() shows a label with no box to draw it in")
		}
	})
}

func TestMeasureLabels(t *testing.T) {
	t.Run("the widest label answers for the alphabet", func(t *testing.T) {
		installMeasurer(t, squareMeasurer{})

		got := badge.MeasureLabels([]string{"A", "WWW", "BB"}, "Menlo", 10, false)
		if want := (badge.LabelMetrics{WidthPerSize: 3, HeightPerSize: 1}); got != want {
			t.Fatalf("MeasureLabels() = %+v, want %+v", got, want)
		}
	})

	t.Run("a platform that cannot measure falls back to the estimate", func(t *testing.T) {
		installMeasurer(t, nil)

		got := badge.MeasureLabels([]string{"AB"}, "Menlo", 10, false)

		wantWidth := float64(badge.EstimateTextWidth("AB", 10)) / 10
		wantHeight := float64(badge.EstimateTextHeight(10)) / 10

		if math.Abs(got.WidthPerSize-wantWidth) > 1e-9 ||
			math.Abs(got.HeightPerSize-wantHeight) > 1e-9 {
			t.Fatalf("MeasureLabels() = %+v, want %v x %v", got, wantWidth, wantHeight)
		}
	})

	t.Run("a failed measurement falls back to the estimate", func(t *testing.T) {
		installMeasurer(t, squareMeasurer{err: errNoTextLayer})

		got := badge.MeasureLabels([]string{"A"}, "Menlo", 10, false)
		if got.WidthPerSize <= 0 || got.HeightPerSize <= 0 {
			t.Fatalf("MeasureLabels() = %+v, want an estimate", got)
		}
	})

	t.Run("no labels measure as nothing, and fit on the multiplier alone", func(t *testing.T) {
		installMeasurer(t, squareMeasurer{})

		got := badge.MeasureLabels([]string{"", ""}, "Menlo", 10, false)
		if got != (badge.LabelMetrics{}) {
			t.Fatalf("MeasureLabels() = %+v, want the zero value", got)
		}
	})
}

func TestLabelMetrics_Repeated(t *testing.T) {
	one := badge.LabelMetrics{WidthPerSize: 0.6, HeightPerSize: 1.2}

	got := one.Repeated(3)
	if math.Abs(got.WidthPerSize-1.8) > 1e-9 || got.HeightPerSize != 1.2 {
		t.Fatalf("Repeated(3) = %+v, want 1.8 wide and 1.2 tall", got)
	}

	if got := one.Repeated(0); got != one {
		t.Fatalf("Repeated(0) = %+v, want one label's worth", got)
	}
}
