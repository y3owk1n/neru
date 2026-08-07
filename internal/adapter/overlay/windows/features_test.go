//go:build windows

package windows

import (
	"image"
	"testing"

	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
)

// Unit tests for the Win32-only rendering decisions. Does not cover GDI
// drawing (see overlay integration tests on WIN-VM), and no longer covers the
// untagged render/badge and render/recursivegrid maths — those tests live
// beside the code they test so they run on every CI leg, not only this one.

// TestShouldShowWinSubKeyPreview pins the threshold for the single preview
// label this backend draws along the bottom of a cell. It stays here because
// it measures the whole cell: Linux and macOS draw a mini-grid instead and
// measure a sub-cell, so this predicate is genuinely this backend's own.
func TestShouldShowWinSubKeyPreview(t *testing.T) {
	t.Parallel()

	cell := image.Rect(0, 0, 30, 30)

	tests := []struct {
		name  string
		style recursivegridcomponent.Style
		want  bool
	}{
		{
			name: "disabled",
			style: recursivegridcomponent.NewStyle(
				recursivegridcomponent.StyleOptions{SubKeyPreview: false},
			),
			want: false,
		},
		{
			name: "enabled without autohide",
			style: recursivegridcomponent.NewStyle(recursivegridcomponent.StyleOptions{
				SubKeyPreview:                   true,
				SubKeyPreviewAutohideMultiplier: 0,
			}),
			want: true,
		},
		{
			name: "enabled cell above threshold",
			style: recursivegridcomponent.NewStyle(recursivegridcomponent.StyleOptions{
				SubKeyPreview:                   true,
				SubKeyPreviewFontSize:           10,
				SubKeyPreviewAutohideMultiplier: 2, // threshold 20, cell 30 passes
			}),
			want: true,
		},
		{
			name: "enabled cell below threshold",
			style: recursivegridcomponent.NewStyle(recursivegridcomponent.StyleOptions{
				SubKeyPreview:                   true,
				SubKeyPreviewFontSize:           20,
				SubKeyPreviewAutohideMultiplier: 2, // threshold 40, cell 30 fails
			}),
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
