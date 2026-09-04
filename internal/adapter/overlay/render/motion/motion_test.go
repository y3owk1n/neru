package motion_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/motion"
)

func TestEaseInOut_ClampsAndIsSymmetric(t *testing.T) {
	t.Parallel()

	const eps = 1e-9

	tests := []struct {
		name     string
		progress float64
		want     float64
	}{
		{"below zero clamps", -1, 0},
		{"zero", 0, 0},
		{"quarter eases in", 0.25, 0.15625},
		{"midpoint", 0.5, 0.5},
		{"three quarters eases out", 0.75, 0.84375},
		{"one", 1, 1},
		{"above one clamps", 2, 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := motion.EaseInOut(testCase.progress)
			if diff := got - testCase.want; diff > eps || diff < -eps {
				t.Errorf("EaseInOut(%v) = %v, want %v", testCase.progress, got, testCase.want)
			}
		})
	}
}

func TestLerpRects_EndpointsAndMidpoint(t *testing.T) {
	t.Parallel()

	from := []image.Rectangle{image.Rect(0, 0, 100, 100)}
	target := []image.Rectangle{image.Rect(50, 50, 150, 250)}

	if got := motion.LerpRects(from, target, 0)[0]; got != from[0] {
		t.Fatalf("t=0: got %v, want %v", got, from[0])
	}

	if got := motion.LerpRects(from, target, 1)[0]; got != target[0] {
		t.Fatalf("t=1: got %v, want %v", got, target[0])
	}

	want := image.Rect(25, 25, 125, 175)
	if got := motion.LerpRects(from, target, 0.5)[0]; got != want {
		t.Fatalf("t=0.5: got %v, want %v", got, want)
	}
}

func TestEased_ContinuingTransitionStaysLinear(t *testing.T) {
	t.Parallel()

	if got := motion.Eased(0.25, false); got != motion.EaseInOut(0.25) {
		t.Fatalf("a fresh transition eases: got %v, want %v", got, motion.EaseInOut(0.25))
	}

	if got := motion.Eased(0.25, true); got != 0.25 {
		t.Fatalf("a continued transition is linear: got %v, want 0.25", got)
	}

	if got := motion.Eased(2, true); got != 1 {
		t.Fatalf("a continued transition clamps: got %v, want 1", got)
	}
}

func TestPointerOrigin_NoPointerBeforeAppearsInPlace(t *testing.T) {
	t.Parallel()

	target, settled, interrupted := image.Pt(1, 1), image.Pt(2, 2), image.Pt(3, 3)

	if got := motion.PointerOrigin(target, settled, interrupted, false, true); got != target {
		t.Fatalf("no pointer before: got %v, want %v", got, target)
	}

	if got := motion.PointerOrigin(target, settled, interrupted, true, true); got != interrupted {
		t.Fatalf("continuing: got %v, want %v", got, interrupted)
	}

	if got := motion.PointerOrigin(target, settled, interrupted, true, false); got != settled {
		t.Fatalf("settled: got %v, want %v", got, settled)
	}
}

func TestLerpPoint_LandsOnTheNearestPixel(t *testing.T) {
	t.Parallel()

	// 0 -> 3 at a third is 1.0 exactly for X; 0 -> 5 at a third is 1.67,
	// which truncation would have put on 1 and rounding puts on 2.
	got := motion.LerpPoint(image.Pt(0, 0), image.Pt(3, 5), 1.0/3.0)
	if want := image.Pt(1, 2); got != want {
		t.Fatalf("LerpPoint = %v, want %v", got, want)
	}
}

func TestLerpRects_MissingOriginIsAlreadyArrived(t *testing.T) {
	t.Parallel()

	target := []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(10, 0, 20, 10)}
	got := motion.LerpRects(target[:1], target, 0.5)

	if got[1] != target[1] {
		t.Fatalf("cell without an origin: got %v, want %v", got[1], target[1])
	}
}

func TestTransitionOrigins_PrefersInterruptedThenLastThenGeometry(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 200, 200)
	target := []image.Rectangle{
		image.Rect(0, 0, 100, 100), image.Rect(100, 0, 200, 100),
		image.Rect(0, 100, 100, 200), image.Rect(100, 100, 200, 200),
	}
	interrupted := []image.Rectangle{
		image.Rect(1, 1, 2, 2), image.Rect(3, 3, 4, 4),
		image.Rect(5, 5, 6, 6), image.Rect(7, 7, 8, 8),
	}
	last := []image.Rectangle{
		image.Rect(9, 9, 10, 10), image.Rect(11, 11, 12, 12),
		image.Rect(13, 13, 14, 14), image.Rect(15, 15, 16, 16),
	}

	got := motion.TransitionOrigins(target, bounds, interrupted, last, bounds)
	if got[0] != interrupted[0] {
		t.Fatalf("interrupted cells win: got %v, want %v", got[0], interrupted[0])
	}

	got = motion.TransitionOrigins(target, bounds, nil, last, bounds)
	if got[0] != last[0] {
		t.Fatalf("last cells next: got %v, want %v", got[0], last[0])
	}

	got = motion.TransitionOrigins(target, bounds, nil, nil, image.Rectangle{})
	if want := image.Rect(50, 50, 50, 50); got[0] != want {
		t.Fatalf("no previous depth collapses to the center: got %v, want %v", got[0], want)
	}

	// Drilling into the bottom-right quarter: each new cell starts at its
	// relative spot inside the cell that was picked, at its final size.
	got = motion.TransitionOrigins(target, bounds, nil, nil, image.Rect(100, 100, 200, 200))
	if want := image.Rect(75, 75, 175, 175); got[0] != want {
		t.Fatalf("zoom origin: got %v, want %v", got[0], want)
	}

	if want := image.Rect(125, 125, 225, 225); got[3] != want {
		t.Fatalf("zoom origin: got %v, want %v", got[3], want)
	}
}
