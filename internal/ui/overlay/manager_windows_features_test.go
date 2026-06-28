//go:build windows

// internal/ui/overlay/manager_windows_features_test.go
// Unit tests for the pure Win32 hint/recursive-grid rendering helpers.
// Does not cover GDI drawing (see overlay integration tests on WIN-VM).

package overlay //nolint:testpackage // tests exercise unexported Win32 rendering helpers directly

import (
	"image"
	"testing"

	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
)

func TestEstimateWinTextWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		fontSize float64
		want     int
	}{
		{"empty", "", 14, 0},
		{"two char label", "AB", 14, 20},    // ceil(2 * 14 * 0.7) = ceil(19.6)
		{"three char label", "ABC", 20, 42}, // 3 * 20 * 0.7 = 42
		{"single char", "A", 10, 7},         // 1 * 10 * 0.7 = 7
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := estimateWinTextWidth(testCase.text, testCase.fontSize); got != testCase.want {
				t.Fatalf(
					"estimateWinTextWidth(%q, %v) = %d, want %d",
					testCase.text,
					testCase.fontSize,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestEstimateWinTextHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fontSize float64
		want     int
	}{
		{"font 14", 14, 20}, // ceil(14 * 1.4) = ceil(19.6)
		{"font 10", 10, 14}, // 10 * 1.4 = 14
		{"font 20", 20, 28}, // 20 * 1.4 = 28
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := estimateWinTextHeight(testCase.fontSize); got != testCase.want {
				t.Fatalf(
					"estimateWinTextHeight(%v) = %d, want %d",
					testCase.fontSize,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestResolveWinAutoPadding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fontSize   float64
		padding    int
		horizontal bool
		want       int
	}{
		{"explicit padding wins", 14, 5, true, 5},
		{"explicit zero padding", 14, 0, false, 0},
		{"auto horizontal large font", 14, -1, true, 8}, // int(14*0.6)=8 > min 6
		{"auto horizontal min floor", 5, -1, true, 6},   // int(5*0.6)=3 -> min 6
		{"auto vertical large font", 14, -1, false, 4},  // int(14*0.35)=4 == min 4
		{"auto vertical min floor", 5, -1, false, 4},    // int(5*0.35)=1 -> min 4
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := resolveWinAutoPadding(testCase.fontSize, testCase.padding, testCase.horizontal)
			if got != testCase.want {
				t.Fatalf("resolveWinAutoPadding(%v, %d, %v) = %d, want %d",
					testCase.fontSize, testCase.padding, testCase.horizontal, got, testCase.want)
			}
		})
	}
}

func TestWinCenteredRect(t *testing.T) {
	t.Parallel()

	cell := image.Rect(0, 0, 100, 100)
	got := winCenteredRect(cell, 20, 10)
	want := image.Rect(40, 45, 60, 55)

	if got != want {
		t.Fatalf("winCenteredRect = %v, want %v", got, want)
	}

	if got.Dx() != 20 || got.Dy() != 10 {
		t.Fatalf("winCenteredRect size = %dx%d, want 20x10", got.Dx(), got.Dy())
	}
}

func TestParseHexColorARGB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  uint32
	}{
		{"full argb", "80FF0000", 0x80FF0000},
		{"hints light background", config.HintsBackgroundColorLight, 0xF2EEF2FF},
		{"hints dark background", config.HintsBackgroundColorDark, 0xF20A1338},
		{"rgb no alpha gets opaque", "#FF0000", 0xFFFF0000},
		{"short form expands", "#F00", 0xFFFF0000},
		{"short form mixed", "#abc", 0xFFAABBCC},
		{"empty is opaque white", "", 0xFFFFFFFF},
		{"invalid length is opaque white", "zzzz", 0xFFFFFFFF},
		{"unparseable is opaque white", "GGGGGG", 0xFFFFFFFF},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := parseHexColorARGB(testCase.value); got != testCase.want {
				t.Fatalf(
					"parseHexColorARGB(%q) = 0x%08X, want 0x%08X",
					testCase.value,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestShouldShowWinSubKeyPreview(t *testing.T) {
	t.Parallel()

	cell := image.Rect(0, 0, 30, 30)

	tests := []struct {
		name  string
		style recursivegridcomponent.Style
		want  bool
	}{
		{
			name:  "disabled",
			style: recursivegridcomponent.Style{SubKeyPreview: false},
			want:  false,
		},
		{
			name: "enabled without autohide",
			style: recursivegridcomponent.Style{
				SubKeyPreview:                   true,
				SubKeyPreviewAutohideMultiplier: 0,
			},
			want: true,
		},
		{
			name: "enabled cell above threshold",
			style: recursivegridcomponent.Style{
				SubKeyPreview:                   true,
				SubKeyPreviewFontSize:           10,
				SubKeyPreviewAutohideMultiplier: 2, // threshold 20, cell 30 passes
			},
			want: true,
		},
		{
			name: "enabled cell below threshold",
			style: recursivegridcomponent.Style{
				SubKeyPreview:                   true,
				SubKeyPreviewFontSize:           20,
				SubKeyPreviewAutohideMultiplier: 2, // threshold 40, cell 30 fails
			},
			want: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldShowWinSubKeyPreview(cell, testCase.style); got != testCase.want {
				t.Fatalf("shouldShowWinSubKeyPreview = %v, want %v", got, testCase.want)
			}
		})
	}
}
