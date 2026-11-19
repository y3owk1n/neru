package ports_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

func TestMeasureText_DefaultRefusesLoudly(t *testing.T) {
	ports.SetTextMeasurer(nil)
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	// A zero size with a nil error would read as "this label takes no room".
	_, err := ports.MeasureText("A", "Anything", 10, false)
	if !derrors.IsNotSupported(err) {
		t.Fatalf("MeasureText() error = %v, want CodeNotSupported", err)
	}
}
