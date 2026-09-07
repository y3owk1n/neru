//go:build linux

package atspi

import (
	"image"
	"testing"

	"go.uber.org/zap"
)

// TestCosmicOrigin_RejectsAFrameOfAnotherSize pins the race guard shared with
// the niri and Sway sources: the compositor's rectangle names the origin only
// for a window the size AT-SPI just measured.
func TestCosmicOrigin_RejectsAFrameOfAnotherSize(t *testing.T) {
	bounds := image.Rect(120, 40, 920, 640)

	cases := []struct {
		name   string
		frame  windowFrame
		wantOK bool
	}{
		{"exact size", windowFrame{Width: 800, Height: 600}, true},
		{
			"within tolerance",
			windowFrame{Width: 800 + windowOriginSizeTolerance, Height: 600},
			true,
		},
		{"another window", windowFrame{Width: 400, Height: 300}, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			origin, accepted, err := cosmicOrigin(bounds, testCase.frame, zap.NewNop())
			if err != nil {
				t.Fatalf("cosmicOrigin() error = %v", err)
			}

			if accepted != testCase.wantOK {
				t.Fatalf("cosmicOrigin() accepted = %v, want %v", accepted, testCase.wantOK)
			}

			if accepted && origin != bounds.Min {
				t.Fatalf("cosmicOrigin() = %v, want %v", origin, bounds.Min)
			}
		})
	}
}
