//go:build windows

package windows

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// TestWinOverlay_DrawHints_ColorsOnlyTheTypedLetters pins a label's colors
// once part of it is typed: the typed letters in the matched text color and
// the rest in the text color, as macOS draws it. Coloring the whole label hid
// which letters were still to type.
func TestWinOverlay_DrawHints_ColorsOnlyTheTypedLetters(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.TextColor = config.Color{Light: "#302505", Dark: "#302505"}
	cfg.Hints.UI.MatchedTextColor = config.Color{Light: "#D4AC3A", Dark: "#D4AC3A"}

	tests := []struct {
		name, typed, want string
	}{
		{name: "nothing typed", typed: "", want: ""},
		{name: "first letter typed", typed: "S", want: "S"},
		{name: "whole label typed", typed: "SDF", want: "SDF"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			window := &recordingWindow{}
			overlay := &winOverlay{window: window, logger: zap.NewNop()}
			hint := hintscomponent.NewHint("SDF", image.Pt(400, 300), image.Pt(40, 20), test.typed)

			overlay.DrawHints(
				[]*hintscomponent.Hint{hint},
				hintscomponent.BuildStyle(cfg.Hints, fixedTheme(false)),
				badge.HintOnTarget,
			)

			if len(window.labels) != 1 {
				t.Fatalf("painted %d hint labels, want 1", len(window.labels))
			}

			label := window.labels[0]
			if label.typed != test.want {
				t.Errorf("typed part = %q, want %q", label.typed, test.want)
			}

			if label.color != 0xFF302505 || label.typedColor != 0xFFD4AC3A {
				t.Errorf("colors = %08X and typed %08X, want FF302505 and typed FFD4AC3A",
					label.color, label.typedColor)
			}
		})
	}
}
