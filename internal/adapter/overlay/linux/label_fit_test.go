//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// TestSharedOverlay_DrawRecursiveGrid_FitsTheLabelToItsCell is #1691 on this
// backend. A label shrinks with its cell rather than vanishing, every label of
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

// TestSharedOverlay_DrawMonitorSelect_FitsTheTextToItsPanel pins that the text
// of a picker panel is fitted like the panel is. The panel was capped at a
// fraction of the monitor and the text was not, so the default 96 point label
// ran out past it on a small monitor.
func TestSharedOverlay_DrawMonitorSelect_FitsTheTextToItsPanel(t *testing.T) {
	t.Parallel()

	style := manager.MonitorSelectStyle{
		FontSize: 96, SubtitleFontSize: 18, PaddingX: -1, PaddingY: -1, BorderRadius: -1,
	}

	tests := []struct {
		name    string
		monitor image.Rectangle
		shrinks bool
	}{
		{name: "a roomy monitor draws the configured size", monitor: image.Rect(0, 0, 1920, 1080)},
		{
			name:    "a small monitor draws a smaller label",
			monitor: image.Rect(0, 0, 1280, 160),
			shrinks: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			surface := &recordingSurface{scale: 1}
			overlay := &sharedOverlay{srf: surface}

			overlay.drawMonitorSelect(
				[]manager.MonitorSelectTarget{{Bounds: test.monitor, Label: "1", Subtitle: "Side"}},
				style,
			)

			label, found := surface.findText("1")
			if !found {
				t.Fatalf("painted %v, want the picker key", surface.paintedStrings())
			}

			if shrunk := label.fontSize < 96; shrunk != test.shrinks {
				t.Fatalf(
					"label drawn at %v, shrunk = %v, want %v",
					label.fontSize,
					shrunk,
					test.shrinks,
				)
			}

			// The panel is the first rounded rectangle, and the label's line
			// has to sit inside it.
			for _, rect := range surface.rects {
				if rect.rounded && float64(rect.bounds.Dy()) < label.fontSize {
					t.Fatalf(
						"a %v point label in a panel %d tall",
						label.fontSize,
						rect.bounds.Dy(),
					)
				}
			}
		})
	}
}
