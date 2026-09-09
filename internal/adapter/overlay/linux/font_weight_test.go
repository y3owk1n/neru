//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// The weight each surface draws at is the one macOS draws it at: hint labels
// bold, everything else regular. Before this was pinned every Linux string was
// bold, so a grid at the default 10 points read heavier than on macOS.

func TestSharedOverlay_DrawHints_LabelsAreBold(t *testing.T) {
	t.Parallel()

	surface := &recordingSurface{scale: 1}
	overlay := &sharedOverlay{srf: surface}

	overlay.drawHints(
		[]*hintscomponent.Hint{
			hintscomponent.NewHint("ab", image.Pt(200, 200), image.Pt(40, 20), ""),
		},
		hintscomponent.BuildStyle(config.DefaultConfig().Hints, nil),
		badge.HintOnTarget,
	)

	painted, found := surface.findText("ab")
	if !found {
		t.Fatalf("painted %v, want the hint label", surface.paintedStrings())
	}

	if !painted.bold {
		t.Error("hint label drawn regular, want bold as on macOS")
	}
}

func TestLinuxOverlayManager_DrawHintSearchInput_IsRegularWeight(t *testing.T) {
	t.Parallel()

	overlayManager, surface := recordingManager()

	err := overlayManager.DrawHintSearchInput(
		"sav", 3,
		hintscomponent.NewSearchInputFrame(image.Pt(10, 20), 300),
		searchStyle(false),
	)
	if err != nil {
		t.Fatalf("DrawHintSearchInput() error = %v", err)
	}

	painted, found := surface.findText("/ sav  3")
	if !found {
		t.Fatalf("painted %v, want the query badge", surface.paintedStrings())
	}

	if painted.bold {
		t.Error("search input drawn bold, want regular as on macOS")
	}
}

func TestLinuxOverlayManager_DrawGrid_LabelsAreRegularWeight(t *testing.T) {
	t.Parallel()

	overlayManager, surface := recordingManager()

	err := overlayManager.DrawGrid(
		domainGrid.NewGrid("ab", image.Rect(0, 0, 800, 600), zap.NewNop()),
		"",
		gridcomponent.BuildStyle(config.DefaultConfig().Grid, fixedTheme(false)),
	)
	if err != nil {
		t.Fatalf("DrawGrid() error = %v", err)
	}

	if len(surface.texts) == 0 {
		t.Fatal("DrawGrid painted no labels")
	}

	for _, painted := range surface.texts {
		if painted.bold {
			t.Fatalf("grid label %q drawn bold, want regular as on macOS", painted.text)
		}
	}
}
