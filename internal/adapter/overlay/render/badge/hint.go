package badge

import "image"

// The geometry `hints.ui.placement` decides, in device pixels. They live here
// rather than in a backend so that the same configured placement puts a badge
// on the same pixel on every backend that reads them — which is the Linux and
// Windows ones. macOS draws the same shapes from its own constants on the
// Objective-C side, and those are a little smaller: an offset badge sits a few
// pixels closer to its element there (docs/CROSS_PLATFORM.md records that; the
// copy is ADR 0007's deliberate exception).
const (
	// HintGap is the gap between an offset hint badge's connector arrow and
	// the target point it labels, so the arrow tip does not sit exactly on the
	// element.
	HintGap = 1

	// HintArrowHeight is the height of the triangular connector arrow drawn
	// between an offset hint badge and its target.
	HintArrowHeight = 5

	// HintArrowHalfBase is half the arrow's base width where it meets the
	// badge edge.
	HintArrowHalfBase = 5

	// HintArrowMinHalfBase is the smallest half-base still worth drawing, and
	// so the flat edge a badge's corner radius has to leave for the arrow to
	// attach to.
	HintArrowMinHalfBase = 2
)

// HintOffset is everything a `hints.ui.placement` value decides about geometry:
// where a hint badge sits relative to the point it labels, and so whether there
// is a connector arrow at all.
//
// Each backend maps the configured placement string onto one of these itself —
// that is where a backend declares which placements it can draw — and the
// drawing math below takes the offset rather than the string, so a placement no
// backend branch names cannot reach a badge.
type HintOffset int

const (
	// HintOnTarget draws the badge over the target point, with no arrow.
	HintOnTarget HintOffset = iota
	// HintAbove draws the badge above the target, with an arrow hanging off
	// its bottom edge and pointing down at the target.
	HintAbove
	// HintBelow draws the badge below the target, with an arrow on its top
	// edge pointing up at the target.
	HintBelow
)

// HintArrow holds the three vertices of a hint connector arrow in screen
// coordinates: the two base corners flush with the badge edge and the tip
// pointing at the target element.
type HintArrow struct {
	BaseLeft  image.Point
	Tip       image.Point
	BaseRight image.Point
}

// HintRadius caps the corner radius of an offset badge so it always keeps a
// flat top/bottom edge wide enough for the connector arrow's base to attach to.
// Without this, a large configured radius (e.g. a full pill) consumes the flat
// edge and the renderer drops the arrow entirely. A badge on its target has no
// arrow, and radii that already leave room are returned unchanged.
func HintRadius(radius, badgeWidth int, offset HintOffset) int {
	if offset == HintOnTarget {
		return radius
	}

	maxRadius := max(badgeWidth/halfDivisor-HintArrowMinHalfBase, 0)

	if radius > maxRadius {
		return maxRadius
	}

	return radius
}

// PlaceHint computes the badge rect for a hint given its target point (the
// element center) and, for an offset badge, the connector arrow that visually
// ties it back to the target.
//
// It is the one implementation of this math: the Linux (Cairo) and Windows
// (GDI) overlays both call it, so a configured placement lands on the same
// pixel on both, and both draw the same shapes as the macOS overlay above the
// difference in constants noted at the top of this file.
//
// radius is the badge's already-resolved corner radius; it keeps the arrow base
// on the badge's flat edge rather than over a rounded corner. The boolean is
// false for a badge drawn on its target, which needs nothing to tie it to one,
// and for a badge too narrow to attach an arrow to at all.
//
// It takes a resolved offset rather than a configured placement string, so
// there is no unrecognized case for it to answer: each backend refuses one
// before a draw begins.
func PlaceHint(
	target image.Point,
	badgeWidth, badgeHeight, radius int,
	offset HintOffset,
) (image.Rectangle, HintArrow, bool) {
	halfW := badgeWidth / halfDivisor
	halfH := badgeHeight / halfDivisor
	centerX := target.X

	var centerY int

	switch offset {
	case HintOnTarget:
		// The badge covers the target, so there is nothing for an arrow to
		// point at.
		return CenteredOn(target, badgeWidth, badgeHeight), HintArrow{}, false
	case HintAbove:
		// Badge sits above the target, offset by the gap plus arrow height so
		// the arrow has room to point down at the target.
		centerY = target.Y - HintGap - HintArrowHeight - halfH
	case HintBelow:
		// Badge sits below the target; arrow points up at the target.
		centerY = target.Y + HintGap + HintArrowHeight + halfH
	}

	// halfW and halfH stay because the arrow and the top/bottom offset are
	// measured from them, not from the rectangle.
	badgeRect := CenteredOn(image.Pt(centerX, centerY), badgeWidth, badgeHeight)

	// Keep the arrow base within the badge's flat edge (inside the corner
	// radius). The caller caps the radius (see HintRadius) so a flat edge
	// remains; only a degenerately narrow badge leaves no room, in which case
	// the arrow is dropped rather than collapsed onto the corners.
	halfBase := HintArrowHalfBase
	if limit := halfW - max(radius, 0); halfBase > limit {
		halfBase = limit
	}

	if halfBase < 1 {
		return badgeRect, HintArrow{}, false
	}

	// Only the two offset placements reach here, and they are mirror images:
	// the arrow leaves the edge of the badge that faces the target.
	baseY, tipY := badgeRect.Min.Y, badgeRect.Min.Y-HintArrowHeight
	if offset == HintAbove {
		baseY, tipY = badgeRect.Max.Y, badgeRect.Max.Y+HintArrowHeight
	}

	arrow := HintArrow{
		BaseLeft:  image.Pt(centerX-halfBase, baseY),
		Tip:       image.Pt(centerX, tipY),
		BaseRight: image.Pt(centerX+halfBase, baseY),
	}

	return badgeRect, arrow, true
}
