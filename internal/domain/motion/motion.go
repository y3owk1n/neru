// Package motion is the velocity model behind the held-key glide. A set of
// held direction keys becomes one direction vector, and a fixed-rate tick
// integrates a speed that may ramp toward a cap into a subpixel cursor position.
package motion

import (
	"image"
	"math"
	"slices"
	"time"
)

// Direction is a heading. FromDelta gives a binding's unit heading, and
// Combine sums the held keys' headings. The integrator scales any non-zero
// heading to unit length, so a diagonal is no faster than a straight line.
type Direction struct {
	X, Y float64
}

// IsZero reports whether no axis is active.
func (d Direction) IsZero() bool {
	return d.X == 0 && d.Y == 0
}

// cancelTolerance is the largest axis sum Combine treats as canceled. Unit
// headings that cancel can leave a rounding residue near 1e-16, depending on
// the order they are summed, and it must not glide.
const cancelTolerance = 1e-9

// Combine sums the distinct headings, so opposite keys cancel, two straight
// keys make a diagonal, and two keys bound to the same step count once.
func Combine(dirs ...Direction) Direction {
	var sum Direction

	for i, dir := range dirs {
		if slices.Contains(dirs[:i], dir) {
			continue
		}

		sum.X += dir.X
		sum.Y += dir.Y
	}

	if math.Abs(sum.X) < cancelTolerance {
		sum.X = 0
	}

	if math.Abs(sum.Y) < cancelTolerance {
		sum.Y = 0
	}

	return sum
}

// FromDelta returns the unit heading of a discrete step, so a held key glides
// at the angle its taps move.
func FromDelta(deltaX, deltaY int) Direction {
	length := math.Hypot(float64(deltaX), float64(deltaY))
	if length == 0 {
		return Direction{}
	}

	return Direction{X: float64(deltaX) / length, Y: float64(deltaY) / length}
}

// unit scales a non-zero heading to length one.
func (d Direction) unit() (float64, float64) {
	length := math.Hypot(d.X, d.Y)

	return d.X / length, d.Y / length
}

// Ramp is what the held_repeat accel options say, before a binding's step
// turns it into speeds. A step is worth Interval of travel, and the speed
// ramps to Multiplier times that over Duration and holds there.
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
// A zero Duration reaches the cap at once. A Multiplier under 1 is treated
// as 1, so speed never ramps down.
func (r Ramp) ParamsFor(stepPx float64) Params {
	base := stepPx / r.Interval.Seconds()
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

// Smoothing is the time constant for easing the velocity toward its target.
// A glide from rest reaches 95% of its speed after three of them, and a turn
// bends over the same span instead of changing heading in one tick.
const Smoothing = 40 * time.Millisecond

// Integrator carries the subpixel position and velocity between ticks.
// Bounds clamp the position, and an empty rectangle means no clamp.
type Integrator struct {
	bounds image.Rectangle
	x, y   float64
	// speed is the ramped target speed, and vx, vy the velocity easing toward
	// it along the held heading.
	speed  float64
	vx, vy float64
	// movedX, movedY is the travel the last Step commanded, before clamping.
	movedX, movedY float64
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
// post. Params travel with every step because the held set decides them. A
// second key with a larger step raises the floor mid-hold. The velocity eases
// toward the target over Smoothing, so motion builds up from rest and a turn
// follows a curve. A zero direction stops at once and returns the integrator
// to rest.
func (i *Integrator) Step(params Params, dir Direction, elapsed time.Duration) image.Point {
	if dir.IsZero() {
		i.speed, i.vx, i.vy = 0, 0, 0
		i.movedX, i.movedY = 0, 0

		return i.Position()
	}

	secs := elapsed.Seconds()
	i.speed = math.Min(math.Max(i.speed+params.Acceleration*secs, params.Speed), params.MaxSpeed)

	unitX, unitY := dir.unit()
	blend := 1 - math.Exp(-secs/Smoothing.Seconds())
	i.vx += (unitX*i.speed - i.vx) * blend
	i.vy += (unitY*i.speed - i.vy) * blend

	i.movedX, i.movedY = i.vx*secs, i.vy*secs
	i.x += i.movedX
	i.y += i.movedY
	i.clamp()

	return i.Position()
}

// Moved is the travel the last Step commanded, before clamping, so a key
// held against a screen edge counts as having moved.
func (i *Integrator) Moved() (float64, float64) {
	return i.movedX, i.movedY
}

// Nudge moves distance pixels in dir at once, outside the velocity ramp,
// and returns the position to post. It measures distance along the heading,
// not per axis.
func (i *Integrator) Nudge(dir Direction, distance float64) image.Point {
	if dir.IsZero() {
		return i.Position()
	}

	unitX, unitY := dir.unit()
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
// edge does not bank up distance that the next reversal has to unwind. It
// also drops the velocity pushing into the edge, so a reversal there leaves
// at once instead of first easing that velocity away.
func (i *Integrator) clamp() {
	if i.bounds.Empty() {
		return
	}

	minX, maxX := float64(i.bounds.Min.X), float64(i.bounds.Max.X-1)
	minY, maxY := float64(i.bounds.Min.Y), float64(i.bounds.Max.Y-1)

	if i.x <= minX && i.vx < 0 || i.x >= maxX && i.vx > 0 {
		i.vx = 0
	}

	if i.y <= minY && i.vy < 0 || i.y >= maxY && i.vy > 0 {
		i.vy = 0
	}

	i.x = math.Max(minX, math.Min(i.x, maxX))
	i.y = math.Max(minY, math.Min(i.y, maxY))
}
