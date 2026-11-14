//go:build linux && cgo

package linux

import (
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
)

// paintedLabel is the name paints gives the label the hints below draw.
const paintedLabel = "text SD"

// drawOneHint paints a single centered hint with cfg's hint style on a
// recording surface at scale, and returns the surface.
func drawOneHint(t *testing.T, cfg *config.Config, scale float64) *recordingSurface {
	t.Helper()

	surface := &recordingSurface{scale: scale}
	overlay := &sharedOverlay{srf: surface}

	overlay.drawHints(
		[]*hintscomponent.Hint{
			hintscomponent.NewHint("SD", image.Pt(200, 200), image.Pt(80, 30), ""),
		},
		hintscomponent.BuildStyle(cfg.Hints, nil),
		badge.HintOnTarget,
	)

	if len(surface.badges) != 1 {
		t.Fatalf("drew %d badges, want 1", len(surface.badges))
	}

	return surface
}

func TestSharedOverlay_DrawHints_LeavesTheBadgeFlatAndUnshadowedByDefault(t *testing.T) {
	t.Parallel()

	surface := drawOneHint(t, config.DefaultConfig(), 1)

	if got := surface.badges[0]; got.fillBottom != got.fill {
		t.Errorf("badge fills %08X to %08X, want a flat fill", got.fill, got.fillBottom)
	}

	if len(surface.masks) != 0 {
		t.Errorf(
			"painted %d masks, want no shadow without hints.ui.shadow_color",
			len(surface.masks),
		)
	}

	if want := []string{paintedBadge, paintedLabel}; !slices.Equal(surface.paints, want) {
		t.Errorf("paints = %v, want %v", surface.paints, want)
	}
}

func TestSharedOverlay_DrawHints_FadesTheBadgeToTheGradientColor(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.BackgroundColor = config.Color{Light: "#FFF785", Dark: "#FFF785"}
	cfg.Hints.UI.BackgroundGradientColor = config.Color{Light: "#FFC542", Dark: "#FFC542"}

	got := drawOneHint(t, cfg, 1).badges[0]

	if got.fill != 0xFFFFF785 || got.fillBottom != 0xFFFFC542 {
		t.Errorf("badge fills %08X to %08X, want FFFFF785 to FFFFC542", got.fill, got.fillBottom)
	}
}

// The shadow is painted before its badge, so the badge covers it, and lands
// where the mask says it does, moved by the offset in device pixels.
func TestSharedOverlay_DrawHints_PaintsTheShadowUnderTheBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scale float64
	}{
		{name: "standard density", scale: 1},
		{name: "HiDPI", scale: 2},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.UI.ShadowColor = config.Color{Light: "#4D000000", Dark: "#4D000000"}
			cfg.Hints.UI.ShadowOffsetX = 1
			cfg.Hints.UI.ShadowOffsetY = 3
			cfg.Hints.UI.ShadowBlur = 7

			surface := drawOneHint(t, cfg, testCase.scale)

			if want := []string{
				paintedMask,
				paintedBadge,
				paintedLabel,
			}; !slices.Equal(
				surface.paints,
				want,
			) {
				t.Fatalf("paints = %v, want %v", surface.paints, want)
			}

			painted := surface.masks[0]
			if painted.color != 0x4D000000 {
				t.Errorf("shadow color = %08X, want 4D000000", painted.color)
			}

			if len(painted.alpha) != painted.bounds.Dx()*painted.bounds.Dy() {
				t.Fatalf("mask holds %d values for %v", len(painted.alpha), painted.bounds)
			}

			// A badge of the same size blurred by the same device blur lands
			// where the draw's shadow must, since the corner radius changes
			// the mask's contents and not its extent.
			badgeRect := surface.badges[0].bounds

			var cache badge.ShadowCache

			mask := cache.HintShadow(badgeRect, 0, badge.HintArrow{}, false, 7*testCase.scale)
			offset := image.Pt(int(1*testCase.scale), int(3*testCase.scale))

			if want := mask.Rect(badgeRect.Min, offset); painted.bounds != want {
				t.Errorf("shadow lands on %v, want %v", painted.bounds, want)
			}
		})
	}
}

func TestSharedOverlay_DrawHints_PaintsTheTextShadowBelowTheLabel(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.TextShadowColor = config.Color{Light: "#99FFFFFF", Dark: "#99FFFFFF"}
	cfg.Hints.UI.TextShadowOffsetY = 1

	surface := drawOneHint(t, cfg, 2)

	if want := []string{
		paintedBadge,
		paintedLabel,
		paintedLabel,
	}; !slices.Equal(
		surface.paints,
		want,
	) {
		t.Fatalf("paints = %v, want %v", surface.paints, want)
	}

	shadow, label := surface.texts[0], surface.texts[1]
	if shadow.color != 0x99FFFFFF {
		t.Errorf("text shadow color = %08X, want 99FFFFFF", shadow.color)
	}

	// One point down is two device pixels at scale 2.
	if got := shadow.center.Sub(label.center); got != image.Pt(0, 2) {
		t.Errorf("text shadow sits %v from the label, want (0,2)", got)
	}
}
