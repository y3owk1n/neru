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

// TestWinOverlay_DrawHints_BadgeGrowsWithTheMonitorScale pins the fix for a
// 150% display showing hints a third too small. The window's DPI scale sizes
// the badge a hint draws, so the same config draws the same apparent size on
// every monitor. The hint's position is a physical pixel and stays put. Only
// the badge around it grows.
func TestWinOverlay_DrawHints_BadgeGrowsWithTheMonitorScale(t *testing.T) {
	t.Parallel()

	const scale = 2.0

	badgeAt := func(windowScale float64) image.Rectangle {
		window := &recordingWindow{scale: windowScale}
		overlay := &winOverlay{window: window, logger: zap.NewNop()}
		style := hintscomponent.BuildStyle(config.DefaultConfig().Hints, fixedTheme(false))
		hint := hintscomponent.NewHint("AB", image.Pt(400, 300), image.Pt(40, 20), "")

		overlay.DrawHints([]*hintscomponent.Hint{hint}, style, badge.HintOnTarget)

		if len(window.rects) == 0 {
			t.Fatalf("DrawHints at scale %v painted no badge", windowScale)
		}

		return window.rects[len(window.rects)-1]
	}

	base := badgeAt(1)
	scaled := badgeAt(scale)

	for _, axis := range []struct {
		name       string
		got, want  int
		center     int
		wantCenter int
	}{
		{"width", scaled.Dx(), base.Dx() * scale, (scaled.Min.X + scaled.Max.X) / 2, (base.Min.X + base.Max.X) / 2},
		{"height", scaled.Dy(), base.Dy() * scale, (scaled.Min.Y + scaled.Max.Y) / 2, (base.Min.Y + base.Max.Y) / 2},
	} {
		// The text estimate rounds up per size, so two pixels of slack either way.
		if axis.got < axis.want-2 || axis.got > axis.want+2 {
			t.Errorf("badge %s at scale %v = %d, want about %d (unscaled %d)",
				axis.name, scale, axis.got, axis.want, axis.want/scale)
		}

		if axis.center != axis.wantCenter {
			t.Errorf("badge %s center moved from %d to %d; the target is a physical pixel",
				axis.name, axis.wantCenter, axis.center)
		}
	}
}
