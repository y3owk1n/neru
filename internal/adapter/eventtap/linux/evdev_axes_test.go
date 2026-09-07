//go:build linux

package linux

import "testing"

// The bitmaps are /proc/bus/input/devices "B: ABS=" lines from a user's
// machine: a Bluetooth keyboard turned away for its volume knob, and the
// laptop touchpad the rule exists to turn away.
func TestHasPointerAxes_KeepsVolumeKnobKeyboardsAndRejectsTouchpads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bits  uint64
		touch bool
	}{
		{name: "no absolute axes", bits: 0, touch: false},
		{name: "bluetooth keyboard with ABS_VOLUME", bits: 0x100000000, touch: false},
		{name: "laptop touchpad", bits: 0x2e0800000000003, touch: true},
		{name: "single-touch ABS_Y only", bits: 1 << evdevAbsY, touch: true},
		{name: "multitouch slot only", bits: 1 << evdevAbsMtSlot, touch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasPointerAxes(tt.bits); got != tt.touch {
				t.Fatalf("hasPointerAxes(%#x) = %v, want %v", tt.bits, got, tt.touch)
			}
		})
	}
}
