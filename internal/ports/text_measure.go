package ports

import (
	"sync"

	"github.com/y3owk1n/neru/internal/derrors"
)

// TextMetrics is how much room one line of text takes at one font, in the
// units the size was given in. Width is the advance of the whole string;
// Height is the font's line height (ascent plus descent), not the ink of the
// particular glyphs, so two strings at one font answer the same Height.
type TextMetrics struct {
	Width  float64
	Height float64
}

// TextMeasurer measures text with the platform's own text layer, the one the
// overlays draw with, so a label can be fitted to its box before it is drawn.
//
// Text scales close to linearly with its font size, which is what makes one
// measurement enough. A caller measures once at the configured size, when a
// style is built, and derives every smaller size by arithmetic. Nothing on a
// draw path may call Measure for a string it has not measured before.
//
// Implementations are safe for concurrent use and never hop to a UI thread:
// styles are built on the main thread before its run loop is servicing, and on
// background goroutines afterwards.
type TextMeasurer interface {
	// Measure returns the metrics of text drawn in family at size. family is a
	// resolved family (ports.ResolveFont). It reports derrors.CodeNotSupported
	// when the platform cannot measure, and callers degrade to an estimate.
	Measure(text, family string, size float64, bold bool) (TextMetrics, error)
}

var (
	textMeasurerMu sync.RWMutex
	textMeasurer   TextMeasurer = unsupportedTextMeasurer{}
)

// SetTextMeasurer installs the process-wide TextMeasurer. Call once during
// infrastructure initialization. Passing nil restores the unsupported default.
func SetTextMeasurer(measurer TextMeasurer) {
	textMeasurerMu.Lock()
	defer textMeasurerMu.Unlock()

	if measurer == nil {
		textMeasurer = unsupportedTextMeasurer{}

		return
	}

	textMeasurer = measurer
}

// MeasureText is a convenience wrapper around the active TextMeasurer. It
// reports derrors.CodeNotSupported when none has been installed (e.g. in
// tests).
func MeasureText(text, family string, size float64, bold bool) (TextMetrics, error) {
	textMeasurerMu.RLock()

	m := textMeasurer

	textMeasurerMu.RUnlock()

	return m.Measure(text, family, size, bold)
}

// unsupportedTextMeasurer is the default measurer. It refuses loudly so a
// caller never mistakes a zero size for a measurement.
type unsupportedTextMeasurer struct{}

// Measure reports derrors.CodeNotSupported.
func (unsupportedTextMeasurer) Measure(string, string, float64, bool) (TextMetrics, error) {
	return TextMetrics{}, derrors.New(
		derrors.CodeNotSupported,
		"text measurement is not available on this platform",
	)
}
