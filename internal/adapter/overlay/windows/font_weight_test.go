//go:build windows

package windows

import (
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// The weight each surface draws at is the one macOS draws it at: hint labels
// bold, everything else regular. Before this was pinned every Windows string
// was bold in both renderers, so a grid at the default 10 points read heavier
// than on macOS.

// boldOf reports how the last paint of text was weighted.
func (w *recordingWindow) boldOf(text string) (bool, bool) {
	index := slices.Index(w.texts, text)
	if index < 0 {
		return false, false
	}

	return w.bolds[index], true
}

func TestWinOverlay_DrawHints_LabelsAreBold(t *testing.T) {
	t.Parallel()

	window := &recordingWindow{}
	overlay := &winOverlay{window: window, logger: zap.NewNop()}
	style := hintscomponent.BuildStyle(config.DefaultConfig().Hints, fixedTheme(false))
	hint := hintscomponent.NewHint("AB", image.Pt(400, 300), image.Pt(40, 20), "")

	overlay.DrawHints([]*hintscomponent.Hint{hint}, style, badge.HintOnTarget)

	bold, found := window.boldOf("AB")
	if !found {
		t.Fatalf("painted %v, want the hint label", window.texts)
	}

	if !bold {
		t.Error("hint label drawn regular, want bold as on macOS")
	}
}

func TestWindowsOverlayManager_DrawHintSearchInput_IsRegularWeight(t *testing.T) {
	t.Parallel()

	window := &recordingWindow{}
	overlayManager := &Manager{Base: manager.NewBase(zap.NewNop())}
	overlayManager.win = &winOverlay{window: window, renderMu: &overlayManager.renderMu}

	err := overlayManager.DrawHintSearchInput(
		"sav", 3,
		hintscomponent.NewSearchInputFrame(image.Pt(10, 20), 300),
		hintscomponent.BuildSearchInputStyle(config.DefaultConfig().Hints, fixedTheme(false)),
	)
	if err != nil {
		t.Fatalf("DrawHintSearchInput() error = %v", err)
	}

	// The badge is the one string this draw paints, cursor included.
	if len(window.texts) != 1 {
		t.Fatalf("painted %v, want the query badge alone", window.texts)
	}

	if window.bolds[0] {
		t.Error("search input drawn bold, want regular as on macOS")
	}
}

func TestWindowsOverlayManager_DrawGrid_LabelsAreRegularWeight(t *testing.T) {
	t.Parallel()

	window := &recordingWindow{}
	overlayManager := &Manager{Base: manager.NewBase(zap.NewNop())}
	overlayManager.win = &winOverlay{window: window, renderMu: &overlayManager.renderMu}

	err := overlayManager.DrawGrid(
		testGrid(), "",
		gridcomponent.BuildStyle(config.DefaultConfig().Grid, fixedTheme(false)),
	)
	if err != nil {
		t.Fatalf("DrawGrid() error = %v", err)
	}

	if len(window.texts) == 0 {
		t.Fatal("DrawGrid painted no labels")
	}

	for index, bold := range window.bolds {
		if bold {
			t.Fatalf("grid label %q drawn bold, want regular as on macOS", window.texts[index])
		}
	}
}
