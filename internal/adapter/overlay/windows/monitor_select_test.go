//go:build windows

package windows

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

func TestMonitorSelectPanelLayout_OddPanelWidthKeepsItsSize(t *testing.T) {
	t.Parallel()

	monitor := image.Rect(0, 0, 100, 100)
	style := manager.MonitorSelectStyle{FontSize: 10}

	panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(monitor, "A", "", style)

	// Label 7 px wide, 14 px tall, no padding: a 7x14 panel centered on (50, 50).
	if want := image.Rect(47, 43, 54, 57); panel != want {
		t.Errorf("panel = %v, want %v", panel, want)
	}

	if want := image.Rect(47, 43, 54, 57); labelRect != want {
		t.Errorf("label rect = %v, want %v", labelRect, want)
	}

	if !subtitleRect.Empty() {
		t.Errorf("subtitle rect = %v, want empty", subtitleRect)
	}

	if radius != 0 {
		t.Errorf("radius = %v, want 0", radius)
	}
}

func TestMonitorSelectPanelLayout_PanelHangsOnItsOwnMonitor(t *testing.T) {
	t.Parallel()

	// A second display to the right of the first: the panel must land on it,
	// not on the primary.
	monitor := image.Rect(1920, 0, 3840, 1080)
	style := manager.MonitorSelectStyle{FontSize: 10}

	panel, labelRect, _, _ := monitorSelectPanelLayout(monitor, "A", "", style)

	if want := image.Rect(2877, 533, 2884, 547); panel != want {
		t.Errorf("panel = %v, want %v", panel, want)
	}

	if labelRect != panel {
		t.Errorf("label rect = %v, want the panel %v", labelRect, panel)
	}
}
