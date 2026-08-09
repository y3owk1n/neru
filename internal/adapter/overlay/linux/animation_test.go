//go:build linux && cgo

package linux

import (
	"image"
	"testing"
)

// Unit tests for the pure easing/color helpers used by the Linux mouse-action
// indicator. Native Cairo animation is covered by overlay integration tests.
func TestApplyEasing(t *testing.T) {
	t.Parallel()

	const eps = 1e-9

	tests := []struct {
		name     string
		easing   string
		progress float64
		want     float64
	}{
		{"linear midpoint", easingLinear, 0.5, 0.5},
		{"ease_in midpoint", easingEaseIn, 0.5, 0.125},
		{"ease_out midpoint", easingEaseOut, 0.5, 0.875},
		{"ease_in_out midpoint", easingEaseInOut, 0.5, 0.5},
		{"unknown falls back to ease_out", "wobble", 0.5, 0.875},
		{"clamped low", easingLinear, -1, 0},
		{"clamped high", easingEaseIn, 2, 1},
		{"zero", easingEaseOut, 0, 0},
		{"one", easingEaseOut, 1, 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := applyEasing(testCase.easing, testCase.progress)
			if diff := got - testCase.want; diff > eps || diff < -eps {
				t.Errorf("applyEasing(%q, %v) = %v, want %v",
					testCase.easing, testCase.progress, got, testCase.want)
			}
		})
	}
}

func TestApplyOpacity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		color   uint32
		opacity float64
		want    uint32
	}{
		{"full opacity keeps color", 0xFF804020, 1, 0xFF804020},
		{"zero opacity clears alpha only", 0xFF804020, 0, 0x00804020},
		{"half opacity halves alpha", 0xFF804020, 0.5, 0x80804020},
		{"clamped negative", 0xAB112233, -0.2, 0x00112233},
		{"clamped above one", 0xAB112233, 1.5, 0xAB112233},
		{"rounds alpha", 0xFFAABBCC, 0.25, 0x40AABBCC},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := applyOpacity(testCase.color, testCase.opacity); got != testCase.want {
				t.Errorf("applyOpacity(0x%08X, %v) = 0x%08X, want 0x%08X",
					testCase.color, testCase.opacity, got, testCase.want)
			}
		})
	}
}

func TestMouseActionIndicatorRect(t *testing.T) {
	t.Parallel()

	rect := mouseActionIndicatorRect(image.Pt(100, 100), 36)
	want := image.Rect(82, 82, 118, 118)

	if rect != want {
		t.Fatalf("mouseActionIndicatorRect = %v, want %v", rect, want)
	}

	if rect.Dx() != 36 || rect.Dy() != 36 {
		t.Errorf("indicator rect size = %dx%d, want 36x36", rect.Dx(), rect.Dy())
	}
}

// Every method a Linux overlay backend exposes to the manager survives a nil
// receiver, because the manager dispatches on possibly-nil backend pointers.
// cancelAnimation was the one exception: it took cancelMu straight off the
// receiver.
func TestSharedOverlay_CancelAnimation_NilReceiver(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("cancelAnimation panicked on a nil receiver: %v", recovered)
		}
	}()

	var overlay *sharedOverlay

	overlay.cancelAnimation()
}
