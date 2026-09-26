//go:build windows && (amd64 || arm64)

package windows

import (
	"image"
	"math"
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
// SendInput reads, the same way wheelEvents does for a whole notch's worth of
// pixels.
func notches(count int) uint32 {
	return uint32(int32(count) * wheelDelta)
}

// pixels converts a signed pixel delta into mouseData the same way
// wheelEvents does: scrollPixelsPerNotch pixels are one notch.
func pixels(count int) uint32 {
	return uint32(int32(count) * wheelUnitsPerPixel)
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
			name:   "a notch's worth of pixels up is one positive wheel notch",
			deltaY: scrollPixelsPerNotch,
			want:   []wheelEvent{{flags: mouseeventfWheel, data: notches(1)}},
		},
		{
			name:   "a pixel is a fraction of a notch, not a notch",
			deltaY: 1,
			want:   []wheelEvent{{flags: mouseeventfWheel, data: pixels(1)}},
		},
		{
			name:   "scroll left is a negative hwheel notch",
			deltaX: scrollPixelsPerNotch,
			want:   []wheelEvent{{flags: mouseeventfHWheel, data: notches(-1)}},
		},
		{
			name:   "scroll right is a positive hwheel notch",
			deltaX: -2 * scrollPixelsPerNotch,
			want:   []wheelEvent{{flags: mouseeventfHWheel, data: notches(2)}},
		},
		{
			name:   "both axes send vertical first",
			deltaX: -scrollPixelsPerNotch,
			deltaY: -scrollPixelsPerNotch,
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

// TestDragGlidePath_StepsEvenlyOntoTheTarget pins the shape of a held warp's
// path: even steps, target last, and no step repeating the pixel before it.
func TestDragGlidePath_StepsEvenlyOntoTheTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		from   image.Point
		target image.Point
		steps  int
		want   []image.Point
	}{
		{
			name:   "an even split reaches the target on the last step",
			from:   image.Point{X: 100, Y: 50},
			target: image.Point{X: 500, Y: 250},
			steps:  4,
			want: []image.Point{
				{X: 200, Y: 100},
				{X: 300, Y: 150},
				{X: 400, Y: 200},
				{X: 500, Y: 250},
			},
		},
		{
			name:   "steps rounding onto one pixel collapse to it",
			from:   image.Point{X: 100, Y: 50},
			target: image.Point{X: 102, Y: 50},
			steps:  4,
			want: []image.Point{
				{X: 101, Y: 50},
				{X: 102, Y: 50},
			},
		},
		{
			name:   "a one pixel glide is the target alone",
			from:   image.Point{X: 100, Y: 50},
			target: image.Point{X: 101, Y: 50},
			steps:  4,
			want:   []image.Point{{X: 101, Y: 50}},
		},
		{
			name:   "one step is the target alone",
			from:   image.Point{X: 100, Y: 50},
			target: image.Point{X: 500, Y: 250},
			steps:  1,
			want:   []image.Point{{X: 500, Y: 250}},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := dragGlidePath(testCase.from, testCase.target, testCase.steps)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("dragGlidePath() = %v, want %v", got, testCase.want)
			}
		})
	}

	if got := dragGlidePath(image.Point{X: 1, Y: 2}, image.Point{X: 1, Y: 2}, 4); got != nil {
		t.Fatalf("no distance gives %v, want nil", got)
	}
}

// TestDragGlidePath_NeverStepsBackwards guards a property a drag depends on:
// every step of a diagonal glide moves further along the line than the one
// before it, so a drag never shrinks a selection partway through.
func TestDragGlidePath_NeverStepsBackwards(t *testing.T) {
	t.Parallel()

	from := image.Point{X: 40, Y: 900}

	for _, target := range []image.Point{
		{X: 41, Y: 901},
		{X: 45, Y: 905},
		{X: 52, Y: 888},
		{X: 3000, Y: 20},
		{X: 40, Y: 60},
	} {
		previous := 0.0

		for index, step := range dragGlidePath(from, target, dragGlideSteps) {
			traveled := math.Hypot(
				float64(step.X-from.X),
				float64(step.Y-from.Y),
			)

			if traveled < previous {
				t.Fatalf(
					"glide to %v steps backwards at index %d: %v is %g pixels along, after %g",
					target, index, step, traveled, previous,
				)
			}

			previous = traveled
		}
	}
}

// TestDragApproachPoint_LeavesAShortMoveOntoTheTarget pins the split every drag
// step lands through: neither axis of the move from the approach point to the
// target travels more than dragLandingApproach, which is the comparison Windows
// makes against its mouse thresholds before doubling a move, and that move is
// the one the application reads as the end of the drag.
func TestDragApproachPoint_LeavesAShortMoveOntoTheTarget(t *testing.T) {
	t.Parallel()

	from := image.Point{X: 500, Y: 400}

	for _, target := range []image.Point{
		{X: 4000, Y: 400},
		{X: 500, Y: 12},
		{X: 740, Y: 400},
		{X: 3000, Y: 1800},
		{X: 20, Y: 30},
	} {
		approach, split := dragApproachPoint(from, target)
		if !split {
			t.Fatalf("dragApproachPoint(%v, %v) reported no split", from, target)
		}

		landingX := max(target.X-approach.X, approach.X-target.X)
		landingY := max(target.Y-approach.Y, approach.Y-target.Y)

		if landingX > dragLandingApproach || landingY > dragLandingApproach {
			t.Fatalf(
				"landing move to %v is (%d, %d) pixels, want at most %d on each axis",
				target, landingX, landingY, dragLandingApproach,
			)
		}
	}
}

// TestDragApproachPoint_DoesNotSplitAShortStep pins the other half: a step
// already within dragLandingApproach is one move, not two, so a slow drag does
// not post a zero-length move per step.
func TestDragApproachPoint_DoesNotSplitAShortStep(t *testing.T) {
	t.Parallel()

	from := image.Point{X: 500, Y: 400}

	for _, target := range []image.Point{
		{X: 500, Y: 400},
		{X: 501, Y: 400},
		{X: 503, Y: 402},
		{X: 504, Y: 400},
	} {
		_, split := dragApproachPoint(from, target)
		if split {
			t.Fatalf("dragApproachPoint(%v, %v) split a short step", from, target)
		}
	}
}

// TestAbsoluteCoordinate_RoundTripsToThePixel pins the mapping against the
// floor(value * size / 65536) Windows applies to an absolute mouse event: the
// coordinate must land on the pixel it was made from, at the edges included.
func TestAbsoluteCoordinate_RoundTripsToThePixel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		origin int
		size   int
	}{
		{name: "single 1080p monitor", origin: 0, size: 1920},
		{name: "monitor left of the primary", origin: -2560, size: 5120},
		{name: "4k pair", origin: 0, size: 7680},
		{name: "odd width", origin: -7, size: 1361},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for pixel := test.origin; pixel < test.origin+test.size; pixel++ {
				value := absoluteCoordinate(pixel, test.origin, test.size)

				back := test.origin + int(value)*test.size/absoluteCoordinateRange
				if back != pixel {
					t.Fatalf("pixel %d maps to %d and back to %d", pixel, value, back)
				}
			}
		})
	}
}

func TestAbsoluteCoordinate_ClampsOutsideTheDesktop(t *testing.T) {
	t.Parallel()

	if got := absoluteCoordinate(-50, 0, 1920); got != 0 {
		t.Fatalf("below the desktop = %d, want 0", got)
	}

	if got := absoluteCoordinate(5000, 0, 1920); got != absoluteCoordinate(1919, 0, 1920) {
		t.Fatalf("past the desktop = %d, want the last pixel's value", got)
	}

	if got := absoluteCoordinate(10, 0, 0); got != 0 {
		t.Fatalf("empty desktop = %d, want 0", got)
	}
}
