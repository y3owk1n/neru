//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// TestSharedOverlay_DrawRecursiveGrid_FitsTheLabelToItsCell is #1691 on this
// backend: a label shrinks with its cell rather than vanishing, every label of
// a draw shares the size, and a dense display is fitted in font units, which
// the rule this replaced got wrong by comparing a logical size with a
// device-pixel cell. textPrim is handed the device size, the fitted size times
// the surface scale.
func TestSharedOverlay_DrawRecursiveGrid_FitsTheLabelToItsCell(t *testing.T) {
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
			name:      "a 2x display scales the configured size",
			boundSide: 600,
			scale:     2,
			wantSize:  40,
			wantShown: true,
		},
		{
			name:      "a 2x display fits in font units",
			boundSide: 108,
			scale:     2,
			wantSize:  24,
			wantShown: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			surface := &recordingSurface{scale: test.scale}
			overlay := &sharedOverlay{srf: surface}

			overlay.drawRecursiveGridWithSubKeyPreview(
				image.Rect(0, 0, test.boundSide, test.boundSide), 1,
				"rtyfghvbn", dims, "", domain.GridDimensions{},
				style, recursivegridcomponent.VirtualPointerState{},
				false, 0,
			)

			if !test.wantShown {
				if len(surface.texts) != 0 {
					t.Fatalf("painted %v, want no labels", surface.paintedStrings())
				}

				return
			}

			if len(surface.texts) != dims.CellCount() {
				t.Fatalf("painted %v, want one label per cell", surface.paintedStrings())
			}

			for _, painted := range surface.texts {
				if painted.fontSize != test.wantSize {
					t.Errorf(
						"label %q drawn at %v, want %v",
						painted.text,
						painted.fontSize,
						test.wantSize,
					)
				}
			}
		})
	}
}

// TestLinuxOverlayManager_DrawGrid_FitsAnOversizedLabelToItsCell pins that a
// font_size too large for the grid's cells is drawn smaller, at one size for
// the whole grid, while the default size is drawn as configured.
func TestLinuxOverlayManager_DrawGrid_FitsAnOversizedLabelToItsCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fontSize int
		shrinks  bool
	}{
		{name: "the default size is drawn as configured", fontSize: config.DefaultGridFontSize},
		{name: "a size larger than the cells shrinks", fontSize: 400, shrinks: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig().Grid
			cfg.UI.FontSize = test.fontSize

			overlayManager, surface := recordingManager()

			err := overlayManager.DrawGrid(
				domainGrid.NewGrid("ab", image.Rect(0, 0, 800, 600), zap.NewNop()),
				"",
				gridcomponent.BuildStyle(cfg, fixedTheme(false)),
			)
			if err != nil {
				t.Fatalf("DrawGrid() error = %v", err)
			}

			if len(surface.texts) == 0 {
				t.Fatal("DrawGrid painted no labels")
			}

			first := surface.texts[0]
			for _, painted := range surface.texts {
				if painted.fontSize != first.fontSize {
					t.Fatalf("label %q drawn at %v and %q at %v, want one size for the grid",
						painted.text, painted.fontSize, first.text, first.fontSize)
				}
			}

			if shrunk := first.fontSize < float64(test.fontSize); shrunk != test.shrinks {
				t.Fatalf("labels drawn at %v for font_size %d, shrunk = %v, want %v",
					first.fontSize, test.fontSize, shrunk, test.shrinks)
			}
		})
	}
}
