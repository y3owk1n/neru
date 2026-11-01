package element

import (
	"cmp"
	"image"
	"slices"

	"github.com/y3owk1n/neru/internal/domain"
)

// Orthogonal weights for Toward, from the CSS Spatial Navigation draft's
// distance function. Drifting off the axis costs far more on a horizontal
// move, because a horizontal move is a move along a row.
const (
	horizontalDriftWeight = 30
	verticalDriftWeight   = 2
)

// The bounds on how much more Nearest charges for vertical distance than for
// horizontal. Never less, so a tall region does not favor the row above, and
// at most three times, so a stacked sidebar icon just above still beats a
// button far along the row.
const (
	minVerticalWeight = 1
	maxVerticalWeight = 3
)

// Target is where a snap lands: the element it chose, and the point on it,
// the center of the part of the element inside the search region.
type Target struct {
	Element *Element
	Point   image.Point
}

// candidate is an element as a snap weighs it, clipped to the search region.
type candidate struct {
	elem *Element
	rect image.Rectangle
}

// Nearest picks the element a snap from origin lands on, among the elements
// overlapping region. It reports false when no element overlaps region.
//
// Each element is first clipped to region, so a wide element only partly
// inside lands the cursor on its part inside. An element that holds others,
// such as a row or a list, is a target only when the region has nothing
// smaller. Clipped to a grid cell it covers, its center is the cell's center,
// so it would win every time and the cursor would never move.
//
// Among the rest, an element containing origin beats any that do not, and the
// smallest of those wins. Otherwise the element coming closest to origin wins,
// with the smaller one breaking a tie. Vertical distance counts the region's
// width over its height times as much as horizontal, held between one and
// three. Most interfaces run in rows, and a grid cell wider than it is tall
// left the starting point that much less certain across than down, so an
// element in the same row beats a slightly nearer one in the next.
func Nearest(elements []*Element, origin image.Point, region image.Rectangle) (Target, bool) {
	candidates := clipCandidates(elements, region)
	weight := verticalWeight(region)

	type rank struct {
		covered int
		dist    float64
		area    int
	}

	ranks := make([]rank, len(candidates))
	for idx, cand := range candidates {
		covered := 1
		if origin.In(cand.rect) {
			covered = 0
		}

		ranks[idx] = rank{
			covered,
			weightedDistanceSquared(origin, cand.rect, weight),
			area(cand.rect),
		}
	}

	return firstHoldingNothing(candidates, candidates, func(a, b int) int {
		return cmp.Or(
			cmp.Compare(ranks[a].covered, ranks[b].covered),
			cmp.Compare(ranks[a].dist, ranks[b].dist),
			cmp.Compare(ranks[a].area, ranks[b].area),
		)
	})
}

// Toward picks the element a snap from origin steps to in dir, among the
// elements overlapping region. It reports false when nothing lies that way.
//
// The step starts from the smallest element under origin that holds no other,
// or from origin itself when there is none, so stepping from a toolbar's empty
// space is not ruled out by the toolbar's whole width. Only an element whose
// center lies past that start's leading edge counts, and never one containing
// origin. As for Nearest, an element holding others is a step only when
// nothing smaller lies that way, so stepping down onto a list lands on its
// first item. The winner has the smallest gap along dir plus a weighted gap
// across it, so an element in the same row or column beats a nearer one
// beside it. A tie goes to the element whose center sits closer to the
// start's across the axis, then to the smaller.
func Toward(
	elements []*Element,
	origin image.Point,
	region image.Rectangle,
	dir domain.Direction,
) (Target, bool) {
	all := clipCandidates(elements, region)
	from := stepStart(all, origin)

	type rank struct{ score, skew, area int }

	var (
		steps []candidate
		ranks []rank
	)

	for _, cand := range all {
		if origin.In(cand.rect) || !beyond(cand.rect, from, dir) {
			continue
		}

		score, skew := stepCost(cand.rect, from, dir)
		steps = append(steps, cand)
		ranks = append(ranks, rank{score, skew, area(cand.rect)})
	}

	return firstHoldingNothing(steps, all, func(a, b int) int {
		return cmp.Or(
			cmp.Compare(ranks[a].score, ranks[b].score),
			cmp.Compare(ranks[a].skew, ranks[b].skew),
			cmp.Compare(ranks[a].area, ranks[b].area),
		)
	})
}

// stepStart is the smallest element containing origin that holds no other,
// or a one-pixel box at origin when there is none.
func stepStart(candidates []candidate, origin image.Point) image.Rectangle {
	var under []candidate

	for _, cand := range candidates {
		if origin.In(cand.rect) {
			under = append(under, cand)
		}
	}

	slices.SortStableFunc(under, func(a, b candidate) int {
		return cmp.Compare(area(a.rect), area(b.rect))
	})

	for _, cand := range under {
		if !holdsAnother(cand, candidates) {
			return cand.rect
		}
	}

	return image.Rectangle{Min: origin, Max: origin.Add(image.Pt(1, 1))}
}

// firstHoldingNothing sorts ranked by compare, which takes indices into
// ranked, and returns the best one that holds none of all, or the best one
// when every one holds something.
//
// Only the best few are ever checked for holding, which is what keeps a busy
// page from paying a comparison of every element against every other.
func firstHoldingNothing(
	ranked []candidate,
	all []candidate,
	compare func(a, b int) int,
) (Target, bool) {
	if len(ranked) == 0 {
		return Target{}, false
	}

	order := make([]int, len(ranked))
	for idx := range order {
		order[idx] = idx
	}

	slices.SortStableFunc(order, compare)

	for _, idx := range order {
		if !holdsAnother(ranked[idx], all) {
			return targetOf(ranked[idx]), true
		}
	}

	return targetOf(ranked[order[0]]), true
}

// holdsAnother reports whether another candidate lies inside cand. Of two with
// the same bounds, such as a row and its only cell, the one listed later holds
// the earlier, so the pair still offers a target that holds nothing.
func holdsAnother(cand candidate, all []candidate) bool {
	beforeCand := true

	for _, other := range all {
		if other.elem == cand.elem {
			beforeCand = false

			continue
		}

		if !other.rect.In(cand.rect) {
			continue
		}

		if other.rect != cand.rect || beforeCand {
			return true
		}
	}

	return false
}

func targetOf(cand candidate) Target {
	return Target{Element: cand.elem, Point: center(cand.rect)}
}

// clipCandidates clips each element to region and drops the ones outside it.
func clipCandidates(elements []*Element, region image.Rectangle) []candidate {
	candidates := make([]candidate, 0, len(elements))

	for _, elem := range elements {
		clipped := elem.Bounds().Intersect(region)
		if !clipped.Empty() {
			candidates = append(candidates, candidate{elem: elem, rect: clipped})
		}
	}

	return candidates
}

// beyond reports whether candidate's center lies past from's leading edge in
// dir.
func beyond(candidate, from image.Rectangle, dir domain.Direction) bool {
	mid := center(candidate)

	switch dir {
	case domain.DirectionRight:
		return mid.X >= from.Max.X
	case domain.DirectionLeft:
		return mid.X < from.Min.X
	case domain.DirectionDown:
		return mid.Y >= from.Max.Y
	case domain.DirectionUp:
		return mid.Y < from.Min.Y
	default:
		return false
	}
}

// stepCost scores a step from to candidate in dir: the gap along dir plus
// the weighted gap across it, and how far apart the two centers sit across
// it, for breaking a tie.
func stepCost(candidate, from image.Rectangle, dir domain.Direction) (int, int) {
	fromMid, candMid := center(from), center(candidate)

	switch dir {
	case domain.DirectionRight, domain.DirectionLeft:
		along := max(candidate.Min.X-from.Max.X, from.Min.X-candidate.Max.X, 0)
		across := max(candidate.Min.Y-from.Max.Y, from.Min.Y-candidate.Max.Y, 0)

		return along + horizontalDriftWeight*across, abs(candMid.Y - fromMid.Y)
	case domain.DirectionUp, domain.DirectionDown:
		along := max(candidate.Min.Y-from.Max.Y, from.Min.Y-candidate.Max.Y, 0)
		across := max(candidate.Min.X-from.Max.X, from.Min.X-candidate.Max.X, 0)

		return along + verticalDriftWeight*across, abs(candMid.X - fromMid.X)
	default:
		return 0, 0
	}
}

// verticalWeight is how much more Nearest charges for vertical distance in
// region: its width over its height, held between the two bounds.
func verticalWeight(region image.Rectangle) float64 {
	return min(max(float64(region.Dx())/float64(region.Dy()), minVerticalWeight), maxVerticalWeight)
}

// weightedDistanceSquared is the squared distance from point to the nearest
// point of rect, with the vertical part scaled by weight, and zero when rect
// contains point.
func weightedDistanceSquared(point image.Point, rect image.Rectangle, weight float64) float64 {
	dx := float64(max(rect.Min.X-point.X, 0, point.X-(rect.Max.X-1)))
	dy := weight * float64(max(rect.Min.Y-point.Y, 0, point.Y-(rect.Max.Y-1)))

	return dx*dx + dy*dy
}

func center(rect image.Rectangle) image.Point {
	return image.Point{X: rect.Min.X + rect.Dx()/2, Y: rect.Min.Y + rect.Dy()/2}
}

func area(rect image.Rectangle) int {
	return rect.Dx() * rect.Dy()
}

func abs(value int) int {
	return max(value, -value)
}
