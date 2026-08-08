//go:build linux

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// TestMonitorSelectPanelLayout_OddPanelWidthDropsItsLastPixel pins the rounding
// the panel is placed with. The panel hangs on the monitor's center point as
// center ± half, so a panel sized to an odd width is drawn one pixel narrower
// than that width — here a 7 px wide panel lands 6 px wide. Fitting it into the
// monitor rectangle instead (badge.CenteredIn) would keep that pixel and move
// the panel's right edge, so this test is what stops the two being swapped.
func TestMonitorSelectPanelLayout_OddPanelWidthDropsItsLastPixel(t *testing.T) {
	t.Parallel()

	monitor := image.Rect(0, 0, 100, 100)
	style := manager.MonitorSelectStyle{FontSize: 10}

	panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(
		monitor, "A", "", style, 1,
	)

	// Label 7 px wide, 14 px tall, no padding: a 7x14 panel centered on (50, 50).
	if want := image.Rect(47, 43, 53, 57); panel != want {
		t.Errorf("panel = %v, want %v", panel, want)
	}

	if want := image.Rect(47, 43, 53, 57); labelRect != want {
		t.Errorf("label rect = %v, want %v", labelRect, want)
	}

	if !subtitleRect.Empty() {
		t.Errorf("subtitle rect = %v, want empty", subtitleRect)
	}

	if radius != 0 {
		t.Errorf("radius = %v, want 0", radius)
	}
}
