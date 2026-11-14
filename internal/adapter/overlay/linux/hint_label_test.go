//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// TestSharedOverlay_DrawHints_ColorsOnlyTheTypedLetters pins a label's colors
// once part of it is typed: the typed letters in the matched text color and
// the rest in the text color, as macOS draws it. Coloring the whole label hid
// which letters were still to type.
func TestSharedOverlay_DrawHints_ColorsOnlyTheTypedLetters(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.TextColor = config.Color{Light: "#302505", Dark: "#302505"}
	cfg.Hints.UI.MatchedTextColor = config.Color{Light: "#D4AC3A", Dark: "#D4AC3A"}

	tests := []struct {
		name    string
		typed   string
		matched int
	}{
		{name: "nothing typed", typed: "", matched: 0},
		{name: "first letter typed", typed: "S", matched: 1},
		{name: "whole label typed", typed: "SDF", matched: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			surface := &recordingSurface{scale: 1}
			overlay := &sharedOverlay{srf: surface}

			overlay.drawHints(
				[]*hintscomponent.Hint{
					hintscomponent.NewHint("SDF", image.Pt(200, 200), image.Pt(80, 30), test.typed),
				},
				hintscomponent.BuildStyle(cfg.Hints, nil),
				badge.HintOnTarget,
			)

			label, found := surface.findText("SDF")
			if !found {
				t.Fatalf("label not painted, painted %v", surface.paintedStrings())
			}

			if label.matched != test.matched {
				t.Errorf("typed bytes = %d, want %d", label.matched, test.matched)
			}

			if label.color != 0xFF302505 || label.matchedColor != 0xFFD4AC3A {
				t.Errorf("colors = %08X and typed %08X, want FF302505 and typed FFD4AC3A",
					label.color, label.matchedColor)
			}
		})
	}
}
