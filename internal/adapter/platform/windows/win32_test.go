//go:build windows && (amd64 || arm64)

package windows

import (
	"image"
	"testing"
)

func TestPackMonitorPoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		point image.Point
		want  uintptr
	}{
		{
			name:  "primary monitor coordinate",
			point: image.Pt(1280, 720),
			want:  uintptr(0x000002D000000500),
		},
		{
			name:  "left of primary monitor coordinate",
			point: image.Pt(-1080, 261),
			want:  uintptr(0x00000105FFFFFBC8),
		},
		{
			name:  "right monitor coordinate",
			point: image.Pt(2846, 261),
			want:  uintptr(0x0000010500000B1E),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := packMonitorPoint(testCase.point); got != testCase.want {
				t.Fatalf("packMonitorPoint(%v) = %#x, want %#x", testCase.point, got, testCase.want)
			}
		})
	}
}

// TestUniquelyNamedScreens_SuffixesOnlyRepeatedNames pins the naming rule
// behind Screens. Two displays that both report "Generic PnP Monitor" get
// their device names as suffixes, and a display with its own name keeps it.
func TestUniquelyNamedScreens_SuffixesOnlyRepeatedNames(t *testing.T) {
	t.Parallel()

	screens := uniquelyNamedScreens([]displayMonitor{
		{name: "Generic PnP Monitor", device: `\\.\DISPLAY5`, bounds: image.Rect(0, 0, 3840, 2160)},
		{
			name:   "Generic PnP Monitor",
			device: `\\.\DISPLAY6`,
			bounds: image.Rect(-1600, 0, 0, 2560),
		},
		{name: "DELL U2720Q", device: `\\.\DISPLAY7`, bounds: image.Rect(3840, 0, 7680, 2160)},
	})

	want := []string{
		"Generic PnP Monitor (DISPLAY5)",
		"Generic PnP Monitor (DISPLAY6)",
		"DELL U2720Q",
	}

	if len(screens) != len(want) {
		t.Fatalf("got %d screens, want %d", len(screens), len(want))
	}

	for idx, screen := range screens {
		if screen.Name != want[idx] {
			t.Errorf("screen %d name = %q, want %q", idx, screen.Name, want[idx])
		}
	}

	if screens[1].Bounds != image.Rect(-1600, 0, 0, 2560) {
		t.Errorf("screen 1 bounds = %v, lost with its name", screens[1].Bounds)
	}
}
