// Package motion holds the platform-neutral arithmetic of the recursive-grid
// transition: the easing curve, the interpolation between the cells of two
// depths, and where a transition starts from. The Linux (Cairo) and Windows
// (Direct2D / GDI) backends both drive a frame loop from here, so a depth
// change zooms the same way on each. macOS evaluates the same curve, the
// kCAMediaTimingFunctionEaseInEaseOut control points, on its own 120Hz timer
// (platform/darwin/overlay_darwin.m), and the choices that make its zoom read
// as smooth are written down here beside the arithmetic they belong to.
package motion

import (
	"image"
	"math"
	"time"
)

const (
	// FramesPerSecond is the rate a software-driven transition renders at.
	FramesPerSecond = 120
	// FrameInterval is the time budget of one frame at FramesPerSecond.
	FrameInterval = time.Second / FramesPerSecond

	smoothStep3 = 3
	smoothStep2 = 2
	halfDivisor = 2
)

// EaseInOut applies a smoothstep ease-in-out interpolation to a progress in
// [0,1], clamping outside it.
func EaseInOut(progress float64) float64 {
	if progress <= 0 {
		return 0
	}

	if progress >= 1 {
		return 1
	}

	return progress * progress * (smoothStep3 - smoothStep2*progress)
}

// Eased maps a raw progress in [0,1] onto the transition curve. A transition
// that continues one still running keeps a linear curve: the cells are
// already moving, and easing in again from where they are reads as a
// stutter on every fast keystroke. macOS makes the same choice.
func Eased(rawProgress float64, continuing bool) float64 {
	if continuing {
		return min(max(rawProgress, 0), 1)
	}

	return EaseInOut(rawProgress)
}

// Lerp linearly interpolates between a and b by t.
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// lerpPixel interpolates one coordinate and lands it on the nearest pixel.
// Truncating instead would pull every edge towards the origin by up to a
// pixel, unevenly between the two ends of a cell, which is the jitter that
// made the software transition read as rough.
func lerpPixel(from, to int, progress float64) int {
	return int(math.Round(Lerp(float64(from), float64(to), progress)))
}

// PointerOrigin answers where the virtual pointer starts its ride on a
// transition towards target: where an interrupted zoom last painted it, where
// the last settled frame drew it, or, with no pointer on screen before, at the
// target itself, so it appears in place rather than flying in.
func PointerOrigin(
	target, settled, interrupted image.Point,
	hadPointer, continuing bool,
) image.Point {
	switch {
	case !hadPointer:
		return target
	case continuing:
		return interrupted
	default:
		return settled
	}
}

// LerpPoint interpolates a point by progress, to the nearest pixel.
func LerpPoint(from, to image.Point, progress float64) image.Point {
	return image.Pt(lerpPixel(from.X, to.X, progress), lerpPixel(from.Y, to.Y, progress))
}

// LerpRects interpolates each rectangle of from towards the one at the same
// index of to by progress. The slices are expected to be the same length;
// extra entries in from are ignored and missing ones are taken as already
// arrived.
func LerpRects(from, to []image.Rectangle, progress float64) []image.Rectangle {
	out := make([]image.Rectangle, len(to))

	for idx, dst := range to {
		if idx >= len(from) {
			out[idx] = dst

			continue
		}

		src := from[idx]
		out[idx] = image.Rectangle{
			Min: LerpPoint(src.Min, dst.Min, progress),
			Max: LerpPoint(src.Max, dst.Max, progress),
		}
	}

	return out
}

// TransitionOrigins answers where each cell of the new depth starts its zoom.
//
// The candidates, in order: the cells a previous transition was interrupted
// on, so a fast keystroke continues from where the screen is; the cells the
// last depth drew, when it had the same count; each cell collapsed to its own
// center when nothing was drawn before; and otherwise each new cell placed at
// its relative position inside the bounds the last depth covered, which is
// what makes a drill-down read as a zoom into the picked cell.
func TransitionOrigins(
	toRects []image.Rectangle,
	bounds image.Rectangle,
	interruptedRects, lastRects []image.Rectangle,
	lastBounds image.Rectangle,
) []image.Rectangle {
	from := make([]image.Rectangle, len(toRects))

	if len(interruptedRects) == len(toRects) {
		copy(from, interruptedRects)

		return from
	}

	if len(lastRects) == len(toRects) {
		copy(from, lastRects)

		return from
	}

	if lastBounds.Empty() {
		for idx, rect := range toRects {
			cx := rect.Min.X + rect.Dx()/halfDivisor
			cy := rect.Min.Y + rect.Dy()/halfDivisor
			from[idx] = image.Rect(cx, cy, cx, cy)
		}

		return from
	}

	lastWidth := float64(lastBounds.Dx())
	lastHeight := float64(lastBounds.Dy())
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	for idx, rect := range toRects {
		relX := (float64(rect.Min.X+rect.Dx()/halfDivisor) - float64(bounds.Min.X)) / width
		relY := (float64(rect.Min.Y+rect.Dy()/halfDivisor) - float64(bounds.Min.Y)) / height
		centerX := int(float64(lastBounds.Min.X) + relX*lastWidth)
		centerY := int(float64(lastBounds.Min.Y) + relY*lastHeight)
		halfWidth := rect.Dx() / halfDivisor
		halfHeight := rect.Dy() / halfDivisor
		from[idx] = image.Rect(
			centerX-halfWidth, centerY-halfHeight,
			centerX+halfWidth, centerY+halfHeight,
		)
	}

	return from
}
