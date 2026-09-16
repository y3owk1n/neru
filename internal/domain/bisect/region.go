package bisect

import (
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// MinSide is the smallest a region's side is cut down to, in the pixels the
// screen is measured in. A press that would cut below it keeps the region.
const MinSide = 2

// Cut is one of the eight ways a region is narrowed: a half on one axis, or a
// quadrant on both.
type Cut uint8

const (
	// CutLeft keeps the left half.
	CutLeft Cut = iota
	// CutRight keeps the right half.
	CutRight
	// CutUp keeps the top half.
	CutUp
	// CutDown keeps the bottom half.
	CutDown
	// CutUpLeft keeps the top-left quadrant.
	CutUpLeft
	// CutUpRight keeps the top-right quadrant.
	CutUpRight
	// CutDownLeft keeps the bottom-left quadrant.
	CutDownLeft
	// CutDownRight keeps the bottom-right quadrant.
	CutDownRight
)

// CutNames are the spellings ParseCut accepts, in the order the constants
// are declared.
var CutNames = []string{
	"left", "right", "up", "down",
	"up_left", "up_right", "down_left", "down_right",
}

// String returns the cut's spelling.
func (c Cut) String() string {
	if int(c) < len(CutNames) {
		return CutNames[c]
	}

	return "unknown"
}

// Signs reports which side each axis keeps: -1 the low side, 1 the high side,
// 0 untouched.
func (c Cut) Signs() (int, int) {
	switch c {
	case CutLeft:
		return -1, 0
	case CutRight:
		return 1, 0
	case CutUp:
		return 0, -1
	case CutDown:
		return 0, 1
	case CutUpLeft:
		return -1, -1
	case CutUpRight:
		return 1, -1
	case CutDownLeft:
		return -1, 1
	case CutDownRight:
		return 1, 1
	default:
		return 0, 0
	}
}

// ParseCut reads a cut from its spelling, case-insensitively. A hyphen is
// accepted where the underscore is.
func ParseCut(name string) (Cut, error) {
	wanted := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "-", "_")

	for index, candidate := range CutNames {
		if candidate == wanted {
			return Cut(index), nil
		}
	}

	return 0, derrors.Newf(
		derrors.CodeInvalidInput,
		"invalid direction %q (expected one of %s)",
		name,
		strings.Join(CutNames, ", "),
	)
}

// Region is the bisect state for one session.
type Region struct {
	screen  image.Rectangle
	region  image.Rectangle
	history []image.Rectangle
}

// NewRegion builds a region covering screen.
func NewRegion(screen image.Rectangle) *Region {
	return &Region{screen: screen, region: screen}
}

// Bounds returns the region as it stands.
func (r *Region) Bounds() image.Rectangle {
	return r.region
}

// Screen returns the bounds the session started from.
func (r *Region) Screen() image.Rectangle {
	return r.screen
}

// Center returns the point a click lands on, rounded to the nearest pixel
// inside the region.
func (r *Region) Center() image.Point {
	return center(r.region)
}

// Depth reports how many cuts deep the region is.
func (r *Region) Depth() int {
	return len(r.history)
}

// Apply keeps the half or quadrant cut names and reports whether the region
// changed. An axis already at MinSide is left alone; a cut that changes
// nothing on any axis is refused whole, so it costs no history.
func (r *Region) Apply(cut Cut) bool {
	signX, signY := cut.Signs()
	next := r.region

	if signX != 0 && next.Dx() > MinSide {
		mid := next.Min.X + next.Dx()/2 //nolint:mnd // halving is the cut
		if signX < 0 {
			next.Max.X = mid
		} else {
			next.Min.X = mid
		}
	}

	if signY != 0 && next.Dy() > MinSide {
		mid := next.Min.Y + next.Dy()/2 //nolint:mnd // halving is the cut
		if signY < 0 {
			next.Max.Y = mid
		} else {
			next.Min.Y = mid
		}
	}

	if next.Eq(r.region) {
		return false
	}

	r.history = append(r.history, r.region)
	r.region = next

	return true
}

// Backtrack restores the region before the last cut and reports whether
// there was one.
func (r *Region) Backtrack() bool {
	depth := len(r.history)
	if depth == 0 {
		return false
	}

	r.region = r.history[depth-1]
	r.history = r.history[:depth-1]

	return true
}

// Reset puts the region back over the whole screen.
func (r *Region) Reset() {
	r.region = r.screen
	r.history = r.history[:0]
}

// RemapToNewBounds proportionally remaps the region and its history onto a
// new screen, so a narrowed region survives a display change.
func (r *Region) RemapToNewBounds(newScreen image.Rectangle) {
	if newScreen.Empty() {
		return
	}

	old := r.screen
	r.region = remapRect(r.region, old, newScreen)

	for index := range r.history {
		r.history[index] = remapRect(r.history[index], old, newScreen)
	}

	r.screen = newScreen
}

// center is the middle of rect, on its last pixel rather than past it.
func center(rect image.Rectangle) image.Point {
	if rect.Empty() {
		return rect.Min
	}

	return image.Pt(
		min(rect.Min.X+rect.Dx()/2, rect.Max.X-1),
		min(rect.Min.Y+rect.Dy()/2, rect.Max.Y-1),
	)
}

// remapRect expresses rect's edges as fractions of oldRef and scales them to
// newRef, rounding to nearest.
func remapRect(rect, oldRef, newRef image.Rectangle) image.Rectangle {
	oldW := oldRef.Dx()
	oldH := oldRef.Dy()

	if oldW == 0 || oldH == 0 {
		return newRef
	}

	minX := newRef.Min.X + divRound((rect.Min.X-oldRef.Min.X)*newRef.Dx(), oldW)
	minY := newRef.Min.Y + divRound((rect.Min.Y-oldRef.Min.Y)*newRef.Dy(), oldH)
	maxX := newRef.Min.X + divRound((rect.Max.X-oldRef.Min.X)*newRef.Dx(), oldW)
	maxY := newRef.Min.Y + divRound((rect.Max.Y-oldRef.Min.Y)*newRef.Dy(), oldH)

	// Onto a smaller display both edges of a narrow region can round to the
	// same pixel. A region that is empty draws nothing and has no center, so
	// it keeps MinSide on each axis, moved inward at the far edge.
	return atLeastMinSide(image.Rect(minX, minY, maxX, maxY), newRef)
}

// atLeastMinSide widens rect to MinSide on any axis it has fallen below, kept
// inside bounds.
func atLeastMinSide(rect, bounds image.Rectangle) image.Rectangle {
	if rect.Dx() < MinSide {
		rect.Max.X = rect.Min.X + MinSide
		if rect.Max.X > bounds.Max.X {
			rect.Max.X = bounds.Max.X
			rect.Min.X = max(rect.Max.X-MinSide, bounds.Min.X)
		}
	}

	if rect.Dy() < MinSide {
		rect.Max.Y = rect.Min.Y + MinSide
		if rect.Max.Y > bounds.Max.Y {
			rect.Max.Y = bounds.Max.Y
			rect.Min.Y = max(rect.Max.Y-MinSide, bounds.Min.Y)
		}
	}

	return rect
}

// divRound divides rounding to nearest rather than toward zero.
func divRound(numerator, denominator int) int {
	if denominator == 0 {
		return 0
	}

	half := denominator / 2 //nolint:mnd // rounding half
	if (numerator < 0) != (denominator < 0) {
		return (numerator - half) / denominator
	}

	return (numerator + half) / denominator
}
