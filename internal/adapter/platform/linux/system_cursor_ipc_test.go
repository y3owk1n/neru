//go:build linux

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

func TestHyprlandActiveScreenSelection(t *testing.T) {
	screens := []ports.Screen{
		{Name: "eDP-1", Bounds: image.Rect(1502, 2616, 2868, 3384)},
		{Name: "HDMI-A-1", Bounds: image.Rect(2868, 2616, 5428, 4056)},
	}

	t.Run("cursor on second monitor", func(t *testing.T) {
		got, ok := screenBoundsContaining(screens, image.Pt(3904, 3970))
		if !ok || got != screens[1].Bounds {
			t.Fatalf("got %v, %v; want %v", got, ok, screens[1].Bounds)
		}
	})

	t.Run("cursor outside every screen", func(t *testing.T) {
		_, ok := screenBoundsContaining(screens, image.Pt(0, 0))
		if ok {
			t.Fatal("expected no screen")
		}
	})

	t.Run("focused monitor fallback", func(t *testing.T) {
		monitors := []hyprlandMonitor{
			{Name: "eDP-1", Focused: false},
			{Name: "HDMI-A-1", Focused: true},
		}

		got, ok := focusedMonitorBounds(screens, monitors)
		if !ok || got != screens[1].Bounds {
			t.Fatalf("got %v, %v; want %v", got, ok, screens[1].Bounds)
		}
	})

	t.Run("no focused monitor", func(t *testing.T) {
		_, ok := focusedMonitorBounds(screens, []hyprlandMonitor{{Name: "eDP-1"}})
		if ok {
			t.Fatal("expected no match")
		}
	})
}
