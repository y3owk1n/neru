//go:build linux && !cgo

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

func TestUnsupportedTextMeasurer_Measure_RefusesLoudly(t *testing.T) {
	// A zero size with a nil error would read as "this label takes no room",
	// and every label would then fit every box.
	_, err := NewTextMeasurer().Measure("A", "DejaVu Sans", 10, false)
	if !derrors.IsNotSupported(err) {
		t.Fatalf("Measure() error = %v, want CodeNotSupported", err)
	}
}
