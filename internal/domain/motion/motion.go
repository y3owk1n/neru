// Package motion is the velocity model behind the held-key glide: a set of
// held direction keys becomes one direction vector, and a fixed-rate tick
// integrates a speed that may ramp toward a cap into a subpixel cursor position.
package motion

import (
	"image"
	"math"
	"time"
)

// diagonalScale is cos(45°): the per-axis share of a unit diagonal.
const diagonalScale = math.Sqrt2 / 2

// Direction is the axis-wise sum of the held direction keys, each axis
// clamped to -1..1, so opposite keys cancel and two keys make a diagonal.
type Direction struct {
	X, Y int
}

// IsZero reports whether no axis is active.
func (d Direction) IsZero() bool {
	return d.X == 0 && d.Y == 0
}

// Combine sums directions axis-wise and clamps each axis to -1..1.
func Combine(dirs ...Direction) Direction {
	var sum Direction
	for _, dir := range dirs {
		sum.X += dir.X
		sum.Y += dir.Y
	}

	return Direction{X: clampAxis(sum.X), Y: clampAxis(sum.Y)}
}

// FromDelta reduces a discrete step to the direction it points in.
func FromDelta(dx, dy int) Direction {
	return Direction{X: clampAxis(dx), Y: clampAxis(dy)}
}

func clampAxis(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	default:
		return 0
	}
}

// Ramp is what the held_repeat accel options say, before a binding's step
// turns it into speeds: a step is worth Interval of travel, the speed ramps
// to Multiplier times that over Duration, and holds there.
type Ramp struct {
	Interval   time.Duration
	Multiplier float64
	Duration   time.Duration
}

// Params is a ramp resolved for one step size. Units are pixels and seconds.
type Params struct {
	Speed        float64 // speed the moment motion starts (px/s)
	MaxSpeed     float64 // cap the ramp settles at (px/s)
	Acceleration float64 // gain while a key stays held (px/s^2)
}

// ParamsFor resolves the ramp for a binding that moves stepPx per Interval.
// A zero Duration reaches the cap at once; a Multiplier under 1 is treated
// as 1, so speed never ramps down.
func (r Ramp) ParamsFor(stepPx int) Params {
	base := float64(stepPx) / r.Interval.Seconds()
	maxSpeed := base * math.Max(r.Multiplier, 1)

	if r.Duration <= 0 {
		return Params{Speed: maxSpeed, MaxSpeed: maxSpeed}
	}

	return Params{
		Speed:        base,
		MaxSpeed:     maxSpeed,
		Acceleration: (maxSpeed - base) / r.Duration.Seconds(),
	}
}

// Integrator carries the subpixel position and current speed between ticks.
// Bounds clamp the position; an empty rectangle means no clamp.
type Integrator struct {
	bounds image.Rectangle
	x, y   float64
	speed  float64
}

// NewIntegrator starts at rest on start.
func NewIntegrator(start image.Point, bounds image.Rectangle) *Integrator {
	return &Integrator{
		bounds: bounds,
		x:      float64(start.X),
		y:      float64(start.Y),
	}
}

// Step advances by elapsed in dir under params and returns the position to
// post. Params travel with every step because the held set decides them: a
// second key with a larger step raises the floor mid-hold. A zero direction
// returns the integrator to rest, so the next press ramps from the floor.
func (i *Integrator) Step(params Params, dir Direction, elapsed time.Duration) image.Point {
	if dir.IsZero() {
		i.speed = 0

		return i.Position()
	}

	secs := elapsed.Seconds()
	i.speed = math.Min(math.Max(i.speed+params.Acceleration*secs, params.Speed), params.MaxSpeed)

	unitX, unitY := float64(dir.X), float64(dir.Y)
	if dir.X != 0 && dir.Y != 0 {
		// A diagonal is no faster than a straight line.
		unitX *= diagonalScale
		unitY *= diagonalScale
	}

	i.x += unitX * i.speed * secs
	i.y += unitY * i.speed * secs
	i.clamp()

	return i.Position()
}

// Nudge moves distance pixels in dir at once, outside the velocity ramp,
// and returns the position to post. A diagonal nudge covers distance along
// the diagonal, not per axis.
func (i *Integrator) Nudge(dir Direction, distance float64) image.Point {
	unitX, unitY := float64(dir.X), float64(dir.Y)
	if dir.X != 0 && dir.Y != 0 {
		unitX *= diagonalScale
		unitY *= diagonalScale
	}

	i.x += unitX * distance
	i.y += unitY * distance
	i.clamp()

	return i.Position()
}

// Position is the current subpixel position rounded to the pixel grid.
func (i *Integrator) Position() image.Point {
	return image.Point{X: int(math.Round(i.x)), Y: int(math.Round(i.y))}
}

// clamp keeps the subpixel position inside bounds, so holding a key at an
// edge does not bank up distance that the next reversal has to unwind.
func (i *Integrator) clamp() {
	if i.bounds.Empty() {
		return
	}

	i.x = math.Max(float64(i.bounds.Min.X), math.Min(i.x, float64(i.bounds.Max.X-1)))
	i.y = math.Max(float64(i.bounds.Min.Y), math.Min(i.y, float64(i.bounds.Max.Y-1)))
}
