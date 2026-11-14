//go:build windows

package windows

import (
	"image"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// Vimium's hint gradient, and the name the recording window gives a badge fill.
const (
	vimiumTop    = "#FFF785"
	vimiumBottom = "#FFC542"
	paintedFill  = "fill"
	paintedLabel = "text AB"
)

// drawOneHint paints a single hint with cfg's hint style on a recording
// window at scale, placed at offset, and returns the window.
func drawOneHint(
	t *testing.T,
	cfg *config.Config,
	scale float64,
	offset badge.HintOffset,
) *recordingWindow {
	t.Helper()

	window := &recordingWindow{scale: scale}
	overlay := &winOverlay{window: window, logger: zap.NewNop()}
	style := hintscomponent.BuildStyle(cfg.Hints, fixedTheme(false))
	hint := hintscomponent.NewHint("AB", image.Pt(400, 300), image.Pt(40, 20), "")

	overlay.DrawHints([]*hintscomponent.Hint{hint}, style, offset)

	if len(window.fills) != 1 {
		t.Fatalf("DrawHints painted %d badge fills, want 1", len(window.fills))
	}

	return window
}

func TestWinOverlay_DrawHints_LeavesTheBadgeFlatAndUnshadowedByDefault(t *testing.T) {
	t.Parallel()

	window := drawOneHint(t, config.DefaultConfig(), 1, badge.HintOnTarget)

	if fill := window.fills[0]; fill[0] != fill[1] {
		t.Errorf("badge fills %08X to %08X, want a flat fill", fill[0], fill[1])
	}

	if want := []string{paintedFill, paintedLabel}; !slices.Equal(window.paints, want) {
		t.Errorf("paints = %v, want %v", window.paints, want)
	}
}

func TestWinOverlay_DrawHints_FadesTheBadgeToTheGradientColor(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.BackgroundColor = config.Color{Light: vimiumTop, Dark: vimiumTop}
	cfg.Hints.UI.BackgroundGradientColor = config.Color{Light: vimiumBottom, Dark: vimiumBottom}

	window := drawOneHint(t, cfg, 1, badge.HintOnTarget)

	if got := window.fills[0]; got != [2]uint32{0xFFFFF785, 0xFFFFC542} {
		t.Errorf("badge fills %08X to %08X, want FFFFF785 to FFFFC542", got[0], got[1])
	}
}

// The arrow takes the color of the edge it hangs from, which is what Cairo's
// padded gradient gives it on Linux.
func TestWinOverlay_DrawHints_FillsTheArrowWithItsEdgesColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		offset badge.HintOffset
		want   uint32
	}{
		{name: "badge above its target", offset: badge.HintAbove, want: 0xFFFFC542},
		{name: "badge below its target", offset: badge.HintBelow, want: 0xFFFFF785},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.UI.BackgroundColor = config.Color{Light: vimiumTop, Dark: vimiumTop}
			cfg.Hints.UI.BackgroundGradientColor = config.Color{
				Light: vimiumBottom,
				Dark:  vimiumBottom,
			}

			window := drawOneHint(t, cfg, 1, test.offset)

			if len(window.triangles) == 0 {
				t.Fatal("DrawHints drew no arrow")
			}

			// The border's triangle goes first, and the fill's over it.
			if got := window.triangles[len(window.triangles)-1]; got != test.want {
				t.Errorf("arrow filled %08X, want %08X", got, test.want)
			}
		})
	}
}

// The shadow is painted before its badge, so the badge covers it, and lands
// where the mask says it does, moved by the offset in device pixels.
func TestWinOverlay_DrawHints_PaintsTheShadowUnderTheBadge(t *testing.T) {
	t.Parallel()

	for _, scale := range []float64{1, 2} {
		cfg := config.DefaultConfig()
		cfg.Hints.UI.ShadowColor = config.Color{Light: "#4D000000", Dark: "#4D000000"}
		cfg.Hints.UI.ShadowOffsetX = 1
		cfg.Hints.UI.ShadowOffsetY = 3
		cfg.Hints.UI.ShadowBlur = 7

		window := drawOneHint(t, cfg, scale, badge.HintOnTarget)

		if want := []string{"mask", paintedFill, paintedLabel}; !slices.Equal(window.paints, want) {
			t.Fatalf("scale %v: paints = %v, want %v", scale, window.paints, want)
		}

		painted := window.masks[0]
		if painted.color != 0x4D000000 {
			t.Errorf("scale %v: shadow color = %08X, want 4D000000", scale, painted.color)
		}

		// The corner radius changes a mask's contents and not its extent, so
		// a square-cornered badge of the same size says where it lands.
		badgeRect := window.rects[0]

		var cache badge.ShadowCache

		mask := cache.HintShadow(badgeRect, 0, badge.HintArrow{}, false, 7*scale)

		want := mask.Rect(badgeRect.Min, image.Pt(int(1*scale), int(3*scale)))
		if painted.bounds != want {
			t.Errorf("scale %v: shadow lands on %v, want %v", scale, painted.bounds, want)
		}
	}
}

func TestWinOverlay_DrawHints_PaintsTheTextShadowBelowTheLabel(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.TextShadowColor = config.Color{Light: "#99FFFFFF", Dark: "#99FFFFFF"}
	cfg.Hints.UI.TextShadowOffsetY = 1

	window := drawOneHint(t, cfg, 2, badge.HintOnTarget)

	if want := []string{
		paintedFill,
		paintedLabel,
		paintedLabel,
	}; !slices.Equal(
		window.paints,
		want,
	) {
		t.Fatalf("paints = %v, want %v", window.paints, want)
	}

	if window.textColors[0] != 0x99FFFFFF {
		t.Errorf("text shadow color = %08X, want 99FFFFFF", window.textColors[0])
	}

	// One point down is two device pixels at scale 2.
	if got := window.textRects[0].Min.Sub(window.textRects[1].Min); got != image.Pt(0, 2) {
		t.Errorf("text shadow sits %v from the label, want (0,2)", got)
	}
}
