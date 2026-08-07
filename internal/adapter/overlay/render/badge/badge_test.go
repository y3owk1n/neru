package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
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
		{name: "short RGB expands to primary", in: "#F00", want: 0xFFFF0000},
		{name: "RRGGBB gets opaque alpha", in: "#102030", want: 0xFF102030},
		{name: "RRGGBB primary gets opaque alpha", in: "#FF0000", want: 0xFFFF0000},
		{name: "AARRGGBB kept verbatim", in: "#80102030", want: 0x80102030},
		{name: "no hash accepted", in: "102030", want: 0xFF102030},
		{name: "no hash AARRGGBB accepted", in: "80FF0000", want: 0x80FF0000},
		{name: "whitespace trimmed", in: "  #102030  ", want: 0xFF102030},
		{name: "bad length falls back to white", in: "#1020", want: opaqueWhite},
		{name: "bad length non-hex falls back to white", in: "zzzz", want: opaqueWhite},
		{name: "bad digits fall back to white", in: "#10203G", want: opaqueWhite},
		{name: "unparseable RRGGBB falls back to white", in: "GGGGGG", want: opaqueWhite},
		{name: "empty falls back to white", in: "", want: opaqueWhite},
		{
			name: "hints light background default",
			in:   config.HintsBackgroundColorLight,
			want: 0xF2EEF2FF,
		},
		{
			name: "hints dark background default",
			in:   config.HintsBackgroundColorDark,
			want: 0xF20A1338,
		},
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

	tests := []struct {
		name       string
		fontSize   float64
		padding    int
		horizontal bool
		want       int
	}{
		{name: "explicit padding wins", fontSize: 20, padding: 9, horizontal: true, want: 9},
		{
			name:       "explicit padding wins at 14pt",
			fontSize:   14,
			padding:    5,
			horizontal: true,
			want:       5,
		},
		{name: "explicit zero padding", fontSize: 14, padding: 0, horizontal: false, want: 0},
		{name: "auto horizontal at 20pt", fontSize: 20, padding: -1, horizontal: true, want: 12},
		{name: "auto vertical at 20pt", fontSize: 20, padding: -1, horizontal: false, want: 7},
		{name: "auto horizontal at 14pt", fontSize: 14, padding: -1, horizontal: true, want: 8},
		{
			name:       "auto vertical at 14pt meets the floor",
			fontSize:   14,
			padding:    -1,
			horizontal: false,
			want:       4,
		},
		{name: "auto horizontal floor at 4pt", fontSize: 4, padding: -1, horizontal: true, want: 6},
		{name: "auto vertical floor at 4pt", fontSize: 4, padding: -1, horizontal: false, want: 4},
		{name: "auto horizontal floor at 5pt", fontSize: 5, padding: -1, horizontal: true, want: 6},
		{name: "auto vertical floor at 5pt", fontSize: 5, padding: -1, horizontal: false, want: 4},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := badge.AutoPadding(testCase.fontSize, testCase.padding, testCase.horizontal)
			if got != testCase.want {
				t.Errorf("AutoPadding(%v, %d, %v) = %d, want %d",
					testCase.fontSize, testCase.padding, testCase.horizontal, got, testCase.want)
			}
		})
	}
}

func TestEstimateTextWidth_RuneCountHeuristic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		fontSize float64
		want     int
	}{
		{name: "empty", text: "", fontSize: 14, want: 0},
		{
			name:     "two char label",
			text:     "AB",
			fontSize: 14,
			want:     20,
		}, // ceil(2 * 14 * 0.7) = ceil(19.6)
		{name: "three char label", text: "ABC", fontSize: 20, want: 42}, // 3 * 20 * 0.7 = 42
		{name: "single char", text: "A", fontSize: 10, want: 7},         // 1 * 10 * 0.7 = 7
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := badge.EstimateTextWidth(testCase.text, testCase.fontSize)
			if got != testCase.want {
				t.Errorf("EstimateTextWidth(%q, %v) = %d, want %d",
					testCase.text, testCase.fontSize, got, testCase.want)
			}
		})
	}
}

func TestEstimateTextHeight_LineHeightHeuristic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fontSize float64
		want     int
	}{
		{name: "font 14", fontSize: 14, want: 20}, // ceil(14 * 1.4) = ceil(19.6)
		{name: "font 10", fontSize: 10, want: 14}, // 10 * 1.4 = 14
		{name: "font 20", fontSize: 20, want: 28}, // 20 * 1.4 = 28
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := badge.EstimateTextHeight(testCase.fontSize); got != testCase.want {
				t.Errorf(
					"EstimateTextHeight(%v) = %d, want %d",
					testCase.fontSize,
					got,
					testCase.want,
				)
			}
		})
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

func TestCenteredIn_CentersAndKeepsSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		container     image.Rectangle
		width, height int
		want          image.Rectangle
	}{
		{
			name:      "even container and badge",
			container: image.Rect(0, 0, 100, 100),
			width:     20,
			height:    10,
			want:      image.Rect(40, 45, 60, 55),
		},
		{
			name:      "offset container",
			container: image.Rect(200, 300, 300, 400),
			width:     20,
			height:    10,
			want:      image.Rect(240, 345, 260, 355),
		},
		{
			name:      "odd container truncates toward the origin",
			container: image.Rect(0, 0, 101, 101),
			width:     10,
			height:    10,
			want:      image.Rect(45, 45, 55, 55),
		},
		{
			name:      "odd badge truncates toward the origin",
			container: image.Rect(0, 0, 100, 100),
			width:     11,
			height:    11,
			want:      image.Rect(45, 45, 56, 56),
		},
		{
			name:      "badge larger than container overhangs symmetrically",
			container: image.Rect(0, 0, 10, 10),
			width:     30,
			height:    20,
			want:      image.Rect(-10, -5, 20, 15),
		},
		{
			name:      "negative container coordinates",
			container: image.Rect(-100, -100, -50, -50),
			width:     10,
			height:    10,
			want:      image.Rect(-80, -80, -70, -70),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := badge.CenteredIn(testCase.container, testCase.width, testCase.height)
			if got != testCase.want {
				t.Errorf("CenteredIn(%v, %d, %d) = %v, want %v",
					testCase.container, testCase.width, testCase.height, got, testCase.want)
			}

			if got.Dx() != testCase.width || got.Dy() != testCase.height {
				t.Errorf("CenteredIn size = %dx%d, want %dx%d",
					got.Dx(), got.Dy(), testCase.width, testCase.height)
			}
		})
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
