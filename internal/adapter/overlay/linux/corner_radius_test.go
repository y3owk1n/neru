//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// recordedRect is one rectangle a shared draw handed to its backend. rounded
// tells the two primitives apart: a shape drawn through rectPrim has no radius
// to report, and that is exactly the bug these tests pin — a corner radius the
// configuration accepts but the draw never passes on.
type recordedRect struct {
	bounds  image.Rectangle
	radius  float64
	rounded bool
}

// recordingSurface is an overlaySurface that draws nothing and remembers what
// it was asked to draw. It stands in for Cairo so these tests need no display,
// and it observes the shared drawing code at the one boundary the backends see.
type recordingSurface struct {
	scale float64
	rects []recordedRect
}

func (s *recordingSurface) surfaceScale() float64 { return s.scale }

func (s *recordingSurface) ensureBuffers() {}

func (s *recordingSurface) beginFrame() bool { return true }

func (s *recordingSurface) surfaceClear() {}

func (s *recordingSurface) clearFrame() {}

func (s *recordingSurface) surfaceClearRect(image.Rectangle) {}

func (s *recordingSurface) surfaceFlush() {}

func (s *recordingSurface) surfaceHide() {}

func (s *recordingSurface) showIndicator() {}

func (s *recordingSurface) finishIndicator() {}

func (s *recordingSurface) syncBeforeAnimation() {}

func (s *recordingSurface) rectPrim(bounds image.Rectangle, _, _ uint32, _ float64) {
	s.rects = append(s.rects, recordedRect{bounds: bounds})
}

func (s *recordingSurface) roundedRectPrim(
	bounds image.Rectangle, radius float64, _, _ uint32, _ float64,
) {
	s.rects = append(s.rects, recordedRect{bounds: bounds, radius: radius, rounded: true})
}

func (s *recordingSurface) hintBadgePrim(
	image.Rectangle, float64, int, badge.HintArrow, uint32, uint32, float64,
) {
}

func (s *recordingSurface) textPrim(_, _ string, _, _, _ float64, _ uint32) {}

// TestSharedOverlay_DrawHints_BoundaryHighlightHonoursItsBorderRadius pins
// hints.boundary_highlight.border_radius to the shape Linux draws. The expected
// radii are the ones macOS (overlay_darwin.m, auto = 4 clamped to half the
// shorter side) and Windows (winAutoRadiusBoundaryCap) resolve the same
// configuration to: a boundary that is a rounded pill on two platforms and a
// square box on the third is the same config producing two different screens.
func TestSharedOverlay_DrawHints_BoundaryHighlightHonoursItsBorderRadius(t *testing.T) {
	t.Parallel()

	// A 100x40 element: half its shorter side is 20, so the auto radius caps at
	// 4 and an oversized configured radius clamps to 20.
	const (
		elementWidth  = 100
		elementHeight = 40
	)

	center := image.Pt(200, 200)
	wantBounds := image.Rect(150, 180, 250, 220)

	tests := []struct {
		name       string
		configured int
		wantRadius float64
	}{
		{name: "auto resolves to the boundary cap", configured: -1, wantRadius: 4},
		{name: "zero keeps sharp corners", configured: 0, wantRadius: 0},
		{name: "a configured radius is drawn", configured: 8, wantRadius: 8},
		{name: "an oversized radius clamps to the shape", configured: 999, wantRadius: 20},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.BoundaryHighlight.Enabled = true
			cfg.Hints.BoundaryHighlight.BorderRadius = testCase.configured

			surface := &recordingSurface{scale: 1}
			overlay := &sharedOverlay{srf: surface}

			overlay.drawHints(
				[]*hintscomponent.Hint{
					hintscomponent.NewHint(
						"a", center, image.Pt(elementWidth, elementHeight), "",
					),
				},
				hintscomponent.BuildStyle(cfg.Hints, nil),
				badge.HintOnTarget,
			)

			if len(surface.rects) != 1 {
				t.Fatalf("drew %d rectangles, want 1 (the boundary highlight)", len(surface.rects))
			}

			boundary := surface.rects[0]
			if boundary.bounds != wantBounds {
				t.Errorf("boundary bounds = %v, want %v", boundary.bounds, wantBounds)
			}

			if !boundary.rounded {
				t.Fatal("boundary was drawn with the sharp-cornered primitive, " +
					"so its configured radius reaches nothing")
			}

			if boundary.radius != testCase.wantRadius {
				t.Errorf("boundary radius = %v, want %v", boundary.radius, testCase.wantRadius)
			}
		})
	}
}

// TestSharedOverlay_DrawFrame_LabelBackgroundHonoursItsBorderRadius pins
// recursive_grid.ui.label_background_border_radius the same way. The auto value
// is a full pill here rather than a capped corner: that is what the shared
// resolver documents for a label background and what Windows draws with it.
// macOS resolves the same -1 to MIN(height/2, 6) in drawGridLabel: — these
// cases assert the resolver Linux now shares with Windows, not agreement with
// that third copy.
func TestSharedOverlay_DrawFrame_LabelBackgroundHonoursItsBorderRadius(t *testing.T) {
	t.Parallel()

	// Font 20 with explicit padding gives a 20x32 plate: half its shorter side
	// is 10, which is both the full-pill radius and the clamp ceiling.
	const (
		labelFontSize = 20
		paddingX      = 3
		paddingY      = 2
	)

	cell := image.Rect(0, 0, 200, 200)
	wantBounds := image.Rect(90, 84, 110, 116)

	tests := []struct {
		name       string
		configured int
		wantRadius float64
	}{
		{name: "auto resolves to a full pill", configured: -1, wantRadius: 10},
		{name: "zero keeps sharp corners", configured: 0, wantRadius: 0},
		{name: "a configured radius is drawn", configured: 4, wantRadius: 4},
		{name: "an oversized radius clamps to the plate", configured: 999, wantRadius: 10},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			style := recursivegridcomponent.NewStyle(recursivegridcomponent.StyleOptions{
				FontSize:                    labelFontSize,
				LabelBackground:             true,
				LabelBackgroundPaddingX:     paddingX,
				LabelBackgroundPaddingY:     paddingY,
				LabelBackgroundBorderRadius: testCase.configured,
			})

			surface := &recordingSurface{scale: 1}
			overlay := &sharedOverlay{srf: surface}

			overlay.drawFrame(
				[]image.Rectangle{cell},
				[]rune{'A'},
				nil,
				domain.GridDimensions{},
				style,
				recursivegridcomponent.VirtualPointerState{},
			)

			// The cell itself is drawn first, the label plate on top of it.
			if len(surface.rects) != 2 {
				t.Fatalf("drew %d rectangles, want 2 (the cell and its label plate)",
					len(surface.rects))
			}

			plate := surface.rects[1]
			if plate.bounds != wantBounds {
				t.Errorf("label plate bounds = %v, want %v", plate.bounds, wantBounds)
			}

			if !plate.rounded {
				t.Fatal("the label plate was drawn with the sharp-cornered primitive, " +
					"so its configured radius reaches nothing")
			}

			if plate.radius != testCase.wantRadius {
				t.Errorf("label plate radius = %v, want %v", plate.radius, testCase.wantRadius)
			}
		})
	}
}
