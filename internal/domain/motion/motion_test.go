package motion_test

import (
	"image"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/motion"
)

// A 10 px step per 50 ms is 200 px/s; a 3x cap over 200 ms is 600 px/s.
var ramp = motion.Ramp{
	Interval:   50 * time.Millisecond,
	Multiplier: 3,
	Duration:   200 * time.Millisecond,
}

const (
	stepPx = 10
	tick   = 10 * time.Millisecond
)

func TestCombine_OppositeKeysCancel(t *testing.T) {
	tests := []struct {
		name string
		in   []motion.Direction
		want motion.Direction
	}{
		{"left and right cancel", []motion.Direction{{X: -1}, {X: 1}}, motion.Direction{}},
		{
			"down and right make a diagonal",
			[]motion.Direction{{Y: 1}, {X: 1}},
			motion.Direction{X: 1, Y: 1},
		},
		{"two rights stay one right", []motion.Direction{{X: 1}, {X: 1}}, motion.Direction{X: 1}},
		{"nothing held is zero", nil, motion.Direction{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := motion.Combine(tt.in...); got != tt.want {
				t.Errorf("Combine = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRamp_ParamsFor(t *testing.T) {
	tests := []struct {
		name string
		ramp motion.Ramp
		want motion.Params
	}{
		{"linear ramp", ramp, motion.Params{Speed: 200, MaxSpeed: 600, Acceleration: 2000}},
		{
			"zero duration starts at the cap",
			motion.Ramp{Interval: 50 * time.Millisecond, Multiplier: 3},
			motion.Params{Speed: 600, MaxSpeed: 600},
		},
		{
			"multiplier of one never ramps",
			motion.Ramp{Interval: 50 * time.Millisecond, Multiplier: 1, Duration: time.Second},
			motion.Params{Speed: 200, MaxSpeed: 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ramp.ParamsFor(stepPx); got != tt.want {
				t.Errorf("ParamsFor(%d) = %+v, want %+v", stepPx, got, tt.want)
			}
		})
	}
}

func TestIntegrator_Step_RampsToTheCap(t *testing.T) {
	params := ramp.ParamsFor(stepPx)
	integ := motion.NewIntegrator(image.Point{}, image.Rectangle{})

	var last image.Point

	distances := make([]int, 0, 50)

	for range 50 {
		pos := integ.Step(params, motion.Direction{X: 1}, tick)
		distances = append(distances, pos.X-last.X)
		last = pos
	}

	// 600 px/s at the cap is 6 px per 10 ms tick; the first tick is at the
	// 200 px/s floor plus one tick of ramp, so 2 px or so.
	if per := distances[len(distances)-1]; per != 6 {
		t.Errorf("final tick moved %d px, want 6 at the cap", per)
	}

	if first := distances[0]; first < 2 || first > 3 {
		t.Errorf("first tick moved %d px, want the 2 px floor", first)
	}
}

func TestIntegrator_Step_DiagonalIsNoFasterThanStraight(t *testing.T) {
	params := ramp.ParamsFor(stepPx)
	straight := motion.NewIntegrator(image.Point{}, image.Rectangle{})
	diagonal := motion.NewIntegrator(image.Point{}, image.Rectangle{})

	var straightPos, diagonalPos image.Point
	for range 100 {
		straightPos = straight.Step(params, motion.Direction{X: 1}, tick)
		diagonalPos = diagonal.Step(params, motion.Direction{X: 1, Y: 1}, tick)
	}

	if diagonalPos.X != diagonalPos.Y {
		t.Errorf("diagonal drifted: x %d, y %d", diagonalPos.X, diagonalPos.Y)
	}

	// sqrt(2)/2 of the straight distance, within rounding.
	if want := int(
		float64(straightPos.X) * 0.7071,
	); diagonalPos.X < want-1 ||
		diagonalPos.X > want+1 {
		t.Errorf(
			"diagonal x %d, want about %d of straight x %d",
			diagonalPos.X,
			want,
			straightPos.X,
		)
	}
}

func TestIntegrator_Step_ClampsToBounds(t *testing.T) {
	params := ramp.ParamsFor(stepPx)
	bounds := image.Rect(0, 0, 100, 50)
	integ := motion.NewIntegrator(image.Point{X: 95, Y: 10}, bounds)

	var pos image.Point
	for range 100 {
		pos = integ.Step(params, motion.Direction{X: 1}, tick)
	}

	if pos.X != 99 {
		t.Fatalf("x = %d, want pinned at 99", pos.X)
	}

	// Reversing moves away at once: nothing banked up beyond the edge.
	if back := integ.Step(params, motion.Direction{X: -1}, tick); back.X >= 99 {
		t.Errorf("after reversing x = %d, want < 99", back.X)
	}
}

func TestIntegrator_Step_ZeroDirectionRests(t *testing.T) {
	params := ramp.ParamsFor(stepPx)
	integ := motion.NewIntegrator(image.Point{}, image.Rectangle{})

	for range 50 {
		integ.Step(params, motion.Direction{X: 1}, tick)
	}

	rest := integ.Step(params, motion.Direction{}, tick)

	// Restarting after rest moves at the floor again, not the cap.
	next := integ.Step(params, motion.Direction{X: 1}, tick)
	if moved := next.X - rest.X; moved > 3 {
		t.Errorf("first tick after rest moved %d px, want the floor", moved)
	}
}

func TestIntegrator_Nudge_MovesOneStepAtOnce(t *testing.T) {
	integ := motion.NewIntegrator(image.Point{X: 50, Y: 50}, image.Rect(0, 0, 100, 100))

	if pos := integ.Nudge(motion.Direction{X: -1}, stepPx); pos.X != 40 || pos.Y != 50 {
		t.Errorf("Nudge left by %d = %v, want (40, 50)", stepPx, pos)
	}

	if pos := integ.Nudge(motion.Direction{X: 1}, 1000); pos.X != 99 {
		t.Errorf("Nudge past the edge = %v, want clamped at 99", pos)
	}
}
