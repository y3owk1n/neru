package badge

import (
	"image"
	"math"

	"github.com/y3owk1n/neru/internal/ports"
)

// fitFill is how much of a box a fitted label may fill. The rest is margin for
// what one measurement cannot promise at every size: hinting at small sizes,
// and a second renderer whose advances differ a little from the one that
// measured (DirectWrite against GDI on Windows).
const fitFill = 0.9

// LabelMetrics is the room a label takes per unit of font size. Text scales
// close to linearly with its font size, so the metrics measured once at the
// configured size answer for every smaller one.
type LabelMetrics struct {
	WidthPerSize  float64
	HeightPerSize float64
}

// MeasureLabels returns the metrics of the widest of texts, measured with the
// platform's text layer (ports.MeasureText) and estimated the way
// EstimateTextWidth does where that cannot measure. One set of metrics for a
// whole alphabet is what lets every label in a draw share one size.
//
// It asks the platform, so it belongs where a Style is built and never on a
// draw path.
func MeasureLabels(texts []string, family string, size float64, bold bool) LabelMetrics {
	if size <= 0 {
		size = fallbackFontSize
	}

	var widest LabelMetrics

	for _, text := range texts {
		metrics := measureLabel(text, family, size, bold)

		widest.WidthPerSize = max(widest.WidthPerSize, metrics.WidthPerSize)
		widest.HeightPerSize = max(widest.HeightPerSize, metrics.HeightPerSize)
	}

	return widest
}

// measureLabel measures one label, falling back to the estimate on any
// failure. A label that is fitted roughly is still better than one that is
// not fitted.
func measureLabel(text, family string, size float64, bold bool) LabelMetrics {
	if text == "" {
		return LabelMetrics{}
	}

	measured, err := ports.MeasureText(text, family, size, bold)
	if err != nil || measured.Width <= 0 || measured.Height <= 0 {
		return LabelMetrics{
			WidthPerSize:  float64(len([]rune(text))) * textWidthMultiplier,
			HeightPerSize: textHeightMultiplier,
		}
	}

	return LabelMetrics{
		WidthPerSize:  measured.Width / size,
		HeightPerSize: measured.Height / size,
	}
}

// Repeated returns the metrics of count labels' worth of m side by side. Grid
// labels are built from one alphabet at a length only a draw knows, so the
// alphabet's widest character is measured once and repeated here. It ignores
// kerning, which can only make the answer slightly wider.
func (m LabelMetrics) Repeated(count int) LabelMetrics {
	return LabelMetrics{
		WidthPerSize:  m.WidthPerSize * float64(max(count, 1)),
		HeightPerSize: m.HeightPerSize,
	}
}

// FontFit is the one rule for a label whose box is fixed by geometry. The
// configured size is a ceiling, the label shrinks until it fits, and it hides
// only when even the floor does not fit.
type FontFit struct {
	// Requested is the configured font size. A label is never scaled up.
	Requested float64
	// Floor is the smallest size worth drawing. Equal to Requested it means
	// "never shrink": the label is drawn at the configured size or not at all.
	// Zero or less means no floor beyond one whole device pixel.
	Floor float64
	// Multiplier requires the box's short side to be at least Multiplier
	// times the font size. Zero or less disables the requirement.
	Multiplier float64
	// Metrics is the label's measured room per unit of font size. The zero
	// value fits on Multiplier alone.
	Metrics LabelMetrics
	// PadX and PadY are what surrounds the text inside the box on each side,
	// a label plate's padding, in the units Requested is in.
	PadX float64
	PadY float64
}

// SizeIn returns the font size to draw at inside box, and whether to draw at
// all. box is in device pixels and scale is device pixels per font unit, so
// the answer is in the units Requested is in, whatever the display's density.
//
// The size is rounded down to a whole device pixel. Every backend realizes
// whole sizes, and whole sizes keep their font caches small.
func (f FontFit) SizeIn(box image.Rectangle, scale float64) (float64, bool) {
	if f.Requested <= 0 || box.Empty() {
		return 0, false
	}

	if scale <= 0 {
		scale = 1
	}

	width := float64(box.Dx()) / scale
	height := float64(box.Dy()) / scale

	size := f.Requested

	if f.Multiplier > 0 {
		size = min(size, min(width, height)/f.Multiplier)
	}

	if f.Metrics.WidthPerSize > 0 {
		size = min(size, (width*fitFill-f.PadX*paddingSideCount)/f.Metrics.WidthPerSize)
	}

	if f.Metrics.HeightPerSize > 0 {
		size = min(size, (height*fitFill-f.PadY*paddingSideCount)/f.Metrics.HeightPerSize)
	}

	floor := min(f.Floor, f.Requested)
	if size < floor || size <= 0 {
		return 0, false
	}

	// Whole device pixels, but never below the floor the label just cleared.
	size = max(math.Floor(size*scale)/scale, floor)
	if size <= 0 {
		return 0, false
	}

	return size, true
}

// SizeAcross fits one size to several boxes, the smallest deciding. A
// transition holds the size that fits both its first frame and its last, so a
// label neither overflows on the way nor changes font on every frame.
func (f FontFit) SizeAcross(scale float64, boxes ...image.Rectangle) (float64, bool) {
	size, show := 0.0, false

	for _, box := range boxes {
		fitted, ok := f.SizeIn(box, scale)
		if !ok {
			return 0, false
		}

		if !show || fitted < size {
			size = fitted
		}

		show = true
	}

	return size, show
}
