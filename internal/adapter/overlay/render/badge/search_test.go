package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

// TestSearchLabel pins the line a hint-search badge shows against the one the
// macOS overlay draws (`drawSearchInputInRect`, overlay_darwin.m): a prompt
// while nothing has been typed, and the query with its match count once
// something has. A user moving between platforms reads the same badge.
func TestSearchLabel(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		query       string
		resultCount int
		want        string
	}{
		"empty query prompts": {
			query:       "",
			resultCount: 0,
			want:        "/ Search hints",
		},
		"query carries its match count": {
			query:       "sav",
			resultCount: 3,
			want:        "/ sav  3",
		},
		"a query matching nothing still says so": {
			query:       "zzz",
			resultCount: 0,
			want:        "/ zzz  0",
		},
		"the count is not the prompt": {
			query:       "",
			resultCount: 12,
			want:        "/ Search hints",
		},
		"a multi-byte query is carried whole": {
			query:       "naïve",
			resultCount: 1,
			want:        "/ naïve  1",
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := badge.SearchLabel(testCase.query, testCase.resultCount)
			if got != testCase.want {
				t.Errorf("SearchLabel(%q, %d) = %q, want %q",
					testCase.query, testCase.resultCount, got, testCase.want)
			}
		})
	}
}

// TestSearchBounds pins the box the label is painted in. The configured width
// is a floor rather than the width: a query longer than the box would otherwise
// be drawn past its own border, which is the one thing a badge that exists to
// show text must not do to it.
func TestSearchBounds(t *testing.T) {
	t.Parallel()

	// "/ ab  1" is 7 runes, so at font size 14 the shared estimates answer
	// ceil(7*14*0.7) = 69 px of text on a ceil(14*1.4) = 20 px line.
	const label = "/ ab  1"

	tests := map[string]struct {
		position image.Point
		minWidth int
		fontSize float64
		paddingX int
		paddingY int
		want     image.Rectangle
	}{
		"the configured width is a floor": {
			position: image.Pt(10, 20),
			minWidth: 300,
			fontSize: 14,
			paddingX: 4,
			paddingY: 2,
			want:     image.Rect(10, 20, 310, 44),
		},
		"a label wider than the box widens it": {
			position: image.Pt(10, 20),
			minWidth: 0,
			fontSize: 14,
			paddingX: 4,
			paddingY: 2,
			want:     image.Rect(10, 20, 87, 44),
		},
		"auto padding is resolved against the font": {
			position: image.Pt(0, 0),
			minWidth: 0,
			fontSize: 14,
			paddingX: -1,
			paddingY: -1,
			// AutoPadding answers max(int(14*0.6), 6) = 8 horizontally and
			// max(int(14*0.35), 4) = 4 vertically.
			want: image.Rect(0, 0, 85, 28),
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := badge.SearchBounds(
				testCase.position, testCase.minWidth, label,
				testCase.fontSize, testCase.paddingX, testCase.paddingY,
			)
			if got != testCase.want {
				t.Errorf("SearchBounds() = %v, want %v", got, testCase.want)
			}
		})
	}
}
