//go:build linux

package linux

import "testing"

// TestScrollNotches_RoundsToTheNearestNotch pins the pixel-to-notch conversion
// every Linux path shares: nearest whole notch, never fewer than one, and the
// animated schedule lands on the same count so switching smooth_scroll on
// changes when a scroll arrives, never how far it goes.
func TestScrollNotches_RoundsToTheNearestNotch(t *testing.T) {
	const notch = scrollPixelsPerNotch

	tests := []struct {
		pixels int
		want   int
	}{
		{pixels: 1, want: 1},
		{pixels: 29, want: 1},
		{pixels: 30, want: 1},
		{pixels: 44, want: 1},
		{pixels: 45, want: 2},
		{pixels: 50, want: 2},
		{pixels: 59, want: 2},
		{pixels: 60, want: 2},
		{pixels: 500, want: 17},
	}

	for _, testCase := range tests {
		for _, delta := range []int{testCase.pixels, -testCase.pixels} {
			if got := scrollNotches(delta); got != testCase.want {
				t.Errorf("scrollNotches(%d) = %d, want %d", delta, got, testCase.want)
			}

			want := float64(testCase.want * notch)
			if delta < 0 {
				want = -want
			}

			animated := sum(scrollChunks(float64(delta), 20, notch, maxScrollUnitsPerRequest))
			if animated != want {
				t.Errorf("animated %d px travels %v, want %v (the unanimated %d notches)",
					delta, animated, want, testCase.want)
			}
		}
	}
}

// TestUinputScrollRemainder_WholeSendLeavesNothing is the Hyprland regression:
// a default 50 px step is two 30 px notches, and the 10 px of rounding left
// over must not go out again on the virtual pointer as a third notch.
func TestUinputScrollRemainder_WholeSendLeavesNothing(t *testing.T) {
	const scale = 30

	tests := []struct {
		name      string
		delta     int
		total     int
		remaining int
		want      int
	}{
		{name: "every notch sent, rounding left over", delta: -50, total: 2, remaining: 0, want: 0},
		{name: "one of two sent hands on the rest", delta: 50, total: 2, remaining: 1, want: 20},
		{name: "every notch sent, exact", delta: 60, total: 2, remaining: 0, want: 0},
		{name: "nothing sent keeps the whole delta", delta: -50, total: 2, remaining: 2, want: -50},
		{name: "half sent keeps the unsent half", delta: 120, total: 4, remaining: 2, want: 60},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := uinputScrollRemainder(testCase.delta, testCase.total, testCase.remaining, scale)
			if got != testCase.want {
				t.Fatalf("uinputScrollRemainder(%d, %d, %d) = %d, want %d",
					testCase.delta, testCase.total, testCase.remaining, got, testCase.want)
			}
		})
	}
}

// TestWlrootsScrollNotch_StepAndDiscreteAgree pins that the pixel step and
// the discrete count a virtual-pointer notch carries point the same way on
// both axes, so clients reading axis and clients reading axis_value120 scroll
// in the same direction.
func TestWlrootsScrollNotch_StepAndDiscreteAgree(t *testing.T) {
	const step = 30

	tests := []struct {
		name     string
		axis     int
		delta    int
		wantStep int
		wantDisc int
	}{
		{
			name: "vertical scroll down", axis: uinputScrollAxisVertical,
			delta: -50, wantStep: step, wantDisc: 1,
		},
		{
			name: "vertical scroll up", axis: uinputScrollAxisVertical,
			delta: 50, wantStep: -step, wantDisc: -1,
		},
		{
			name: "horizontal scroll right", axis: uinputScrollAxisHorizontal,
			delta: 50, wantStep: step, wantDisc: 1,
		},
		{
			name: "horizontal scroll left", axis: uinputScrollAxisHorizontal,
			delta: -50, wantStep: -step, wantDisc: -1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			gotStep, gotDisc := wlrootsScrollNotch(testCase.axis, testCase.delta, step)
			if gotStep != testCase.wantStep || gotDisc != testCase.wantDisc {
				t.Fatalf("wlrootsScrollNotch(%d, %d) = (%d, %d), want (%d, %d)",
					testCase.axis, testCase.delta,
					gotStep, gotDisc,
					testCase.wantStep, testCase.wantDisc)
			}
		})
	}
}
