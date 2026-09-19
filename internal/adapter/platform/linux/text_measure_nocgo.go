//go:build linux && !cgo

package linux

import (
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewTextMeasurer returns a ports.TextMeasurer that refuses. Cairo is the text
// layer on Linux and a non-CGO build has none, so callers fall back to their
// estimate. CGO builds (the default) get the cairo measurer in
// text_measure_cgo.go.
func NewTextMeasurer() ports.TextMeasurer {
	return unsupportedTextMeasurer{}
}

// unsupportedTextMeasurer reports derrors.CodeNotSupported for every string.
type unsupportedTextMeasurer struct{}

// Measure implements ports.TextMeasurer.
func (unsupportedTextMeasurer) Measure(string, string, float64, bool) (ports.TextMetrics, error) {
	return ports.TextMetrics{}, derrors.New(
		derrors.CodeNotSupported,
		"text measurement requires CGO-enabled Linux builds",
	)
}
