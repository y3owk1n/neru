//go:build windows

package windows

import (
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
)

// sizeOf reports the device-pixel font size the last paint of text used.
func (w *recordingWindow) sizeOf(text string) (float64, bool) {
	for index, painted := range slices.Backward(w.texts) {
		if painted == text {
			return w.sizes[index], true
		}
	}

	return 0, false
}

// TestWinOverlay_DrawRecursiveGrid_FitsTheLabelToItsCell is #1691 on this
// backend: a label shrinks with its cell rather than vanishing, every label of
// a draw shares the size, and a dense display is fitted in font units, which
// the rule this replaced got wrong by comparing a logical size with a
// device-pixel cell.
func TestWinOverlay_DrawRecursiveGrid_FitsTheLabelToItsCell(t *testing.T) {
	t.Parallel()

	style := recursivegridcomponent.NewStyle(recursivegridcomponent.StyleOptions{
		FontSize:                20,
		MinFontSize:             6,
		LabelAutohideMultiplier: 1.5,
	})
	dims := domain.GridDimensions{Cols: 3, Rows: 3}

	tests := []struct {
		name      string
		boundSide int
		scale     float64
		wantSize  float64
		wantShown bool
	}{
		{
			name:      "roomy cells draw the configured size",
			boundSide: 600,
			scale:     1,
			wantSize:  20,
			wantShown: true,
		},
		{
			name:      "tight cells shrink the label",
			boundSide: 54,
			scale:     1,
			wantSize:  12,
			wantShown: true,
		},
		{name: "cells under the floor draw no label", boundSide: 24, scale: 1, wantShown: false},
		{
			name:      "a 150% display scales the configured size",
			boundSide: 600,
			scale:     1.5,
			wantSize:  30,
			wantShown: true,
		},
		{
			name:      "a 150% display fits in font units",
			boundSide: 81,
			scale:     1.5,
			wantSize:  18,
			wantShown: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			window := &recordingWindow{scale: test.scale}
			overlay := &winOverlay{window: window, logger: zap.NewNop()}

			overlay.DrawRecursiveGrid(
				image.Rect(0, 0, test.boundSide, test.boundSide), 1,
				"RTYFGHVBN", dims, "", domain.GridDimensions{},
				style, recursivegridcomponent.VirtualPointerState{},
				false, 0,
			)

			if !test.wantShown {
				if len(window.texts) != 0 {
					t.Fatalf("painted %v, want no labels", window.texts)
				}

				return
			}

			if len(window.texts) != dims.CellCount() {
				t.Fatalf("painted %v, want one label per cell", window.texts)
			}

			for index, size := range window.sizes {
				if size != test.wantSize {
					t.Errorf(
						"label %q drawn at %v, want %v",
						window.texts[index],
						size,
						test.wantSize,
					)
				}
			}

			if _, found := window.sizeOf("G"); !found {
				t.Errorf("painted %v, want the centre key among them", window.texts)
			}
		})
	}
}
