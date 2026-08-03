package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

const opaqueWhite = 0xFFFFFFFF

func TestParseHexARGB_Notations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want uint32
	}{
		{name: "short RGB expands", in: "#abc", want: 0xFFAABBCC},
		{name: "RRGGBB gets opaque alpha", in: "#102030", want: 0xFF102030},
		{name: "AARRGGBB kept verbatim", in: "#80102030", want: 0x80102030},
		{name: "no hash accepted", in: "102030", want: 0xFF102030},
		{name: "whitespace trimmed", in: "  #102030  ", want: 0xFF102030},
		{name: "bad length falls back to white", in: "#1020", want: opaqueWhite},
		{name: "bad digits fall back to white", in: "#10203G", want: opaqueWhite},
		{name: "empty falls back to white", in: "", want: opaqueWhite},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := badge.ParseHexARGB(testCase.in); got != testCase.want {
				t.Errorf("ParseHexARGB(%q) = %#x, want %#x", testCase.in, got, testCase.want)
			}
		})
	}
}

func TestAutoPadding_ExplicitAndAuto(t *testing.T) {
	t.Parallel()

	if got := badge.AutoPadding(20, 9, true); got != 9 {
		t.Errorf("explicit padding = %d, want 9", got)
	}

	if got := badge.AutoPadding(20, -1, true); got != 12 {
		t.Errorf("auto horizontal at 20pt = %d, want 12 (20*0.6)", got)
	}

	if got := badge.AutoPadding(20, -1, false); got != 7 {
		t.Errorf("auto vertical at 20pt = %d, want 7 (20*0.35)", got)
	}

	// Floors apply for tiny fonts.
	if got := badge.AutoPadding(4, -1, true); got != 6 {
		t.Errorf("auto horizontal floor = %d, want 6", got)
	}

	if got := badge.AutoPadding(4, -1, false); got != 4 {
		t.Errorf("auto vertical floor = %d, want 4", got)
	}
}

func TestBounds_AnchorsAndFallbackFontSize(t *testing.T) {
	t.Parallel()

	got := badge.Bounds(100, 200, 10, 20, "ab", 10, 0, 0)

	wantW := badge.EstimateTextWidth("ab", 10)
	wantH := badge.EstimateTextHeight(10)
	want := image.Rect(110, 220, 110+wantW, 220+wantH)

	if got != want {
		t.Errorf("Bounds = %v, want %v", got, want)
	}

	// Non-positive font size must not produce a zero-size badge.
	fallback := badge.Bounds(0, 0, 0, 0, "ab", 0, -1, -1)
	if fallback.Dx() == 0 || fallback.Dy() == 0 {
		t.Errorf("Bounds with zero font size = %v, want non-empty", fallback)
	}
}

func TestBorderRadius_Modes(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 40, 20)

	if got := badge.BorderRadius(0, bounds, 6); got != 0 {
		t.Errorf("zero radius = %v, want 0 (sharp corners)", got)
	}

	if got := badge.BorderRadius(-1, bounds, 6); got != 6 {
		t.Errorf("auto radius with cap = %v, want 6", got)
	}

	if got := badge.BorderRadius(-1, bounds, 0); got != 10 {
		t.Errorf("auto radius uncapped = %v, want 10 (half the smaller side)", got)
	}

	if got := badge.BorderRadius(50, bounds, 0); got != 10 {
		t.Errorf("oversized radius = %v, want clamp to 10", got)
	}

	if got := badge.BorderRadius(4, bounds, 0); got != 4 {
		t.Errorf("explicit radius = %v, want 4", got)
	}
}
