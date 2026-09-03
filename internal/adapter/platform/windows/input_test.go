//go:build windows && (amd64 || arm64)

package windows

import (
	"slices"
	"testing"
	"unsafe"
)

func TestSendInputStructLayout(t *testing.T) {
	t.Parallel()

	if got := unsafe.Sizeof(input{}); got != 40 {
		t.Fatalf("sizeof(input) = %d, want 40", got)
	}

	if got := unsafe.Sizeof(mouseInput{}); got != 32 {
		t.Fatalf("sizeof(mouseInput) = %d, want 32", got)
	}

	if got := unsafe.Offsetof(input{}.mi); got != 8 {
		t.Fatalf("offsetof(input.mi) = %d, want 8", got)
	}

	// The keyboard arm of the union carries its own padding to reach the same
	// 40 bytes cbSize demands, so it needs the same offset check: a wVk landing
	// anywhere but byte 8 is a SendInput that silently posts the wrong key.
	if got := unsafe.Sizeof(keyInput{}); got != 40 {
		t.Fatalf("sizeof(keyInput) = %d, want 40", got)
	}

	if got := unsafe.Sizeof(keyboardInput{}); got != 24 {
		t.Fatalf("sizeof(keyboardInput) = %d, want 24", got)
	}

	if got := unsafe.Offsetof(keyInput{}.ki); got != 8 {
		t.Fatalf("offsetof(keyInput.ki) = %d, want 8", got)
	}
}

// notches converts a signed wheel count into the two's-complement mouseData
// SendInput reads, the same way wheelEvents does.
func notches(count int) uint32 {
	return uint32(int32(count) * wheelDelta)
}

// TestWheelEvents_NegatesHorizontalDelta pins the sign convention across the
// SendInput seam: Neru's positive deltaX means left everywhere, while
// MOUSEEVENTF_HWHEEL reads positive as right, so scroll_left must arrive as
// a negative HWHEEL notch and scroll_right as a positive one.
func TestWheelEvents_NegatesHorizontalDelta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		deltaX int
		deltaY int
		want   []wheelEvent
	}{
		{name: "no movement sends nothing"},
		{
			name:   "scroll up is a positive wheel notch",
			deltaY: 1,
			want:   []wheelEvent{{flags: mouseeventfWheel, data: notches(1)}},
		},
		{
			name:   "scroll left is a negative hwheel notch",
			deltaX: 1,
			want:   []wheelEvent{{flags: mouseeventfHWheel, data: notches(-1)}},
		},
		{
			name:   "scroll right is a positive hwheel notch",
			deltaX: -2,
			want:   []wheelEvent{{flags: mouseeventfHWheel, data: notches(2)}},
		},
		{
			name:   "both axes send vertical first",
			deltaX: -1,
			deltaY: -1,
			want: []wheelEvent{
				{flags: mouseeventfWheel, data: notches(-1)},
				{flags: mouseeventfHWheel, data: notches(1)},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := wheelEvents(testCase.deltaX, testCase.deltaY)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf(
					"wheelEvents(%d, %d) = %+v, want %+v",
					testCase.deltaX,
					testCase.deltaY,
					got,
					testCase.want,
				)
			}
		})
	}
}
