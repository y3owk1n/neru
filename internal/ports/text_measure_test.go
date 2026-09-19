package ports_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// fakeMeasurer is a deterministic ports.TextMeasurer used to exercise the
// global accessor in isolation.
type fakeMeasurer struct{}

func (fakeMeasurer) Measure(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
	return ports.TextMetrics{Width: float64(len(text)) * size, Height: size}, nil
}

func TestMeasureText_DefaultRefusesLoudly(t *testing.T) {
	ports.SetTextMeasurer(nil)
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	// A zero size with a nil error would read as "this label takes no room".
	_, err := ports.MeasureText("A", "Anything", 10, false)
	if !derrors.IsNotSupported(err) {
		t.Fatalf("MeasureText() error = %v, want CodeNotSupported", err)
	}
}

func TestMeasureText_DispatchesToInstalledMeasurer(t *testing.T) {
	ports.SetTextMeasurer(fakeMeasurer{})
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	got, err := ports.MeasureText("AB", "Anything", 10, false)
	if err != nil {
		t.Fatalf("MeasureText() error = %v", err)
	}

	if want := (ports.TextMetrics{Width: 20, Height: 10}); got != want {
		t.Fatalf("MeasureText() = %+v, want %+v", got, want)
	}
}
