//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

func TestLinuxOverlay_DrawRecursiveGrid_SingleLineByDefault(t *testing.T) {
	t.Parallel()

	mgr, surface := recordingManager()
	bounds := image.Rect(0, 0, 100, 100)
	dims := domain.GridDimensions{Cols: 2, Rows: 2}

	cfg := config.DefaultConfig().RecursiveGrid
	style := recursivegridcomponent.BuildStyle(cfg, fixedTheme(false))

	mgr.x11.DrawRecursiveGridWithSubKeyPreview(
		bounds, 0, "abcd", dims, "", domain.GridDimensions{},
		style, recursivegridcomponent.VirtualPointerState{}, false, 0,
	)

	// 2x2 grid should draw exactly 4 rectangles when secondary line is disabled.
	if len(surface.rects) != 4 {
		t.Fatalf("surface.rects count = %d, want 4", len(surface.rects))
	}

	for i, r := range surface.rects {
		if r.border != style.LineColorARGB() {
			t.Errorf("rect[%d].border = %#08x, want %#08x", i, r.border, style.LineColorARGB())
		}
		if r.lineWidth != style.LineWidthF() {
			t.Errorf("rect[%d].lineWidth = %v, want %v", i, r.lineWidth, style.LineWidthF())
		}
	}
}

func TestLinuxOverlay_DrawRecursiveGrid_SecondaryLine(t *testing.T) {
	t.Parallel()

	mgr, surface := recordingManager()
	bounds := image.Rect(0, 0, 200, 200)
	dims := domain.GridDimensions{Cols: 2, Rows: 2}

	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.LineWidth = 2
	cfg.UI.LineColor = config.Color{Light: "#ff0000", Dark: "#ff0000"}
	cfg.UI.SecondaryLineWidth = 4
	cfg.UI.SecondaryLineColor = config.Color{Light: "#00ff00", Dark: "#00ff00"}

	style := recursivegridcomponent.BuildStyle(cfg, fixedTheme(false))
	if !style.HasSecondaryLine() {
		t.Fatal("style.HasSecondaryLine() = false, want true")
	}

	mgr.x11.DrawRecursiveGridWithSubKeyPreview(
		bounds, 0, "abcd", dims, "", domain.GridDimensions{},
		style, recursivegridcomponent.VirtualPointerState{}, false, 0,
	)

	// 2x2 grid has 4 cell background fills + 12 divider line stripes = 16 rects total.
	if len(surface.rects) != 16 {
		t.Fatalf("surface.rects count = %d, want 16 (4 fills + 12 stripes)", len(surface.rects))
	}

	expectedPrimaryColor := badge.ParseHexARGB("#ff0000")
	expectedSecondaryColor := badge.ParseHexARGB("#00ff00")

	// Verify that the 12 divider stripes use either expectedPrimaryColor or expectedSecondaryColor as fill
	dividerRects := surface.rects[4:]
	for i, r := range dividerRects {
		if r.fill != expectedPrimaryColor && r.fill != expectedSecondaryColor {
			t.Errorf("dividerRects[%d].fill = %#08x, want either %#08x (primary) or %#08x (secondary)",
				i, r.fill, expectedPrimaryColor, expectedSecondaryColor)
		}
	}
}

func TestLinuxOverlay_DrawRecursiveGrid_SecondaryLineInheritsPrimaryWidthWhenZero(t *testing.T) {
	t.Parallel()

	mgr, surface := recordingManager()
	bounds := image.Rect(0, 0, 200, 200)
	dims := domain.GridDimensions{Cols: 2, Rows: 2}

	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.LineWidth = 3
	cfg.UI.LineColor = config.Color{Light: "#000000", Dark: "#000000"}
	cfg.UI.SecondaryLineWidth = 0 // 0 means inherit primary LineWidth
	cfg.UI.SecondaryLineColor = config.Color{Light: "#ffffff", Dark: "#ffffff"}

	style := recursivegridcomponent.BuildStyle(cfg, fixedTheme(false))
	if !style.HasSecondaryLine() {
		t.Fatal("style.HasSecondaryLine() = false, want true")
	}

	mgr.x11.DrawRecursiveGridWithSubKeyPreview(
		bounds, 0, "abcd", dims, "", domain.GridDimensions{},
		style, recursivegridcomponent.VirtualPointerState{}, false, 0,
	)

	// 4 fills + 12 stripes = 16 rects
	if len(surface.rects) != 16 {
		t.Fatalf("surface.rects count = %d, want 16 (4 fills + 12 stripes)", len(surface.rects))
	}

	// For line X=0, primary is at [0, 3] (dx=3) and secondary is at [3, 6] (dx=3)
	dividerRects := surface.rects[4:]
	leftPrimary := dividerRects[0]
	leftSecondary := dividerRects[1]

	if leftPrimary.bounds.Dx() != 3 {
		t.Errorf("primary stripe width = %d, want 3", leftPrimary.bounds.Dx())
	}
	if leftSecondary.bounds.Dx() != 3 {
		t.Errorf("secondary stripe width = %d, want 3 (inherited)", leftSecondary.bounds.Dx())
	}
}
