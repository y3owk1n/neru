//go:build linux

package linux //nolint:testpackage // tests exercise unexported render helpers directly

import (
	"image"
	"testing"
)

// Unit tests for the pure hint badge/arrow placement math shared by the X11 and
// Wayland overlays. Native Cairo rendering is covered by overlay integration
// tests; here we only assert the geometry the C draw calls receive.
func TestHintBadgePlacement_CenterHasNoArrow(t *testing.T) {
	target := image.Pt(100, 100)
	badge, arrow, hasArrow := hintBadgePlacement(target, 40, 20, 4, "center")

	if hasArrow {
		t.Fatalf("center placement should not draw an arrow, got %+v", arrow)
	}
	// Center placement keeps the badge centered on the target.
	if got := badge.Min.X + badge.Dx()/2; got != target.X {
		t.Errorf("badge center X = %d, want %d", got, target.X)
	}

	if got := badge.Min.Y + badge.Dy()/2; got != target.Y {
		t.Errorf("badge center Y = %d, want %d", got, target.Y)
	}
}

func TestHintBadgePlacement_UnknownPlacementHasNoArrow(t *testing.T) {
	if _, _, hasArrow := hintBadgePlacement(image.Pt(10, 10), 40, 20, 4, "floating"); hasArrow {
		t.Fatal("unrecognized placement should not draw an arrow")
	}
}

func TestHintBadgePlacement_Bottom(t *testing.T) {
	target := image.Pt(200, 150)
	badgeWidth, badgeHeight, radius := 40, 20, 4
	badge, arrow, hasArrow := hintBadgePlacement(target, badgeWidth, badgeHeight, radius, "bottom")

	if !hasArrow {
		t.Fatal("bottom placement should draw an arrow")
	}

	// Badge sits below the target, offset by gap + arrow height + half height.
	wantTop := target.Y + hintPlacementGap + hintArrowHeight
	if badge.Min.Y != wantTop {
		t.Errorf("badge top = %d, want %d", badge.Min.Y, wantTop)
	}

	if got := badge.Min.X + badge.Dx()/2; got != target.X {
		t.Errorf(
			"badge is not horizontally centered on target: center X = %d, want %d",
			got,
			target.X,
		)
	}

	// The arrow base is flush with the badge top edge and the tip points up at
	// the target, landing one gap short of it.
	if arrow.baseLeft.Y != badge.Min.Y || arrow.baseRight.Y != badge.Min.Y {
		t.Errorf("arrow base not on badge top edge: baseLeft=%v baseRight=%v top=%d",
			arrow.baseLeft, arrow.baseRight, badge.Min.Y)
	}

	if arrow.tip.Y >= arrow.baseLeft.Y {
		t.Errorf(
			"bottom-placement arrow tip should point up (above base): tip=%v base=%v",
			arrow.tip,
			arrow.baseLeft,
		)
	}

	if arrow.tip.X != target.X {
		t.Errorf("arrow tip X = %d, want %d (aligned with target)", arrow.tip.X, target.X)
	}

	if wantTipY := badge.Min.Y - hintArrowHeight; arrow.tip.Y != wantTipY {
		t.Errorf("arrow tip Y = %d, want %d", arrow.tip.Y, wantTipY)
	}

	assertArrowSymmetry(t, arrow, target.X)
}

func TestHintBadgePlacement_Top(t *testing.T) {
	target := image.Pt(200, 150)
	badgeWidth, badgeHeight, radius := 40, 20, 4
	badge, arrow, hasArrow := hintBadgePlacement(target, badgeWidth, badgeHeight, radius, "top")

	if !hasArrow {
		t.Fatal("top placement should draw an arrow")
	}

	// Badge sits above the target.
	wantBottom := target.Y - hintPlacementGap - hintArrowHeight
	if badge.Max.Y != wantBottom {
		t.Errorf("badge bottom = %d, want %d", badge.Max.Y, wantBottom)
	}

	// The arrow base is flush with the badge bottom edge and the tip points down
	// at the target.
	if arrow.baseLeft.Y != badge.Max.Y || arrow.baseRight.Y != badge.Max.Y {
		t.Errorf("arrow base not on badge bottom edge: baseLeft=%v baseRight=%v bottom=%d",
			arrow.baseLeft, arrow.baseRight, badge.Max.Y)
	}

	if arrow.tip.Y <= arrow.baseLeft.Y {
		t.Errorf(
			"top-placement arrow tip should point down (below base): tip=%v base=%v",
			arrow.tip,
			arrow.baseLeft,
		)
	}

	if wantTipY := badge.Max.Y + hintArrowHeight; arrow.tip.Y != wantTipY {
		t.Errorf("arrow tip Y = %d, want %d", arrow.tip.Y, wantTipY)
	}

	assertArrowSymmetry(t, arrow, target.X)
}

func TestHintBadgePlacement_ArrowBaseClampedToFlatEdge(t *testing.T) {
	// A narrow badge with a large radius leaves little flat edge; the arrow base
	// must clamp inside the corner radius rather than run onto the rounded
	// corners or overflow the badge.
	target := image.Pt(50, 50)
	badgeWidth, badgeHeight, radius := 14, 20, 6

	badge, arrow, hasArrow := hintBadgePlacement(target, badgeWidth, badgeHeight, radius, "bottom")
	if !hasArrow {
		t.Fatal("expected an arrow")
	}

	halfBase := arrow.baseRight.X - target.X
	if maxHalf := badge.Dx()/2 - radius; halfBase > maxHalf {
		t.Errorf("arrow half-base %d exceeds flat-edge limit %d", halfBase, maxHalf)
	}

	if arrow.baseLeft.X < badge.Min.X || arrow.baseRight.X > badge.Max.X {
		t.Errorf("arrow base %v..%v overflows badge %v", arrow.baseLeft, arrow.baseRight, badge)
	}
}

func TestHintTailEdge(t *testing.T) {
	target := image.Pt(200, 150)

	badge, arrow, hasArrow := hintBadgePlacement(target, 40, 20, 4, "bottom")
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailTop {
		t.Errorf(
			"bottom placement (arrow points up) should merge the tail into the top edge, got %d",
			edge,
		)
	}

	badge, arrow, hasArrow = hintBadgePlacement(target, 40, 20, 4, "top")
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailBottom {
		t.Errorf(
			"top placement (arrow points down) should merge the tail into the bottom edge, got %d",
			edge,
		)
	}

	badge, arrow, hasArrow = hintBadgePlacement(target, 40, 20, 4, "center")
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailNone {
		t.Errorf("center placement should have no tail, got %d", edge)
	}
}

func TestHintBadgeRadius_ReservesTailFlatEdge(t *testing.T) {
	// Center placement is never capped — the tail doesn't exist there.
	if got := hintBadgeRadius(100, 40, "center"); got != 100 {
		t.Errorf("center radius should be unchanged, got %d", got)
	}

	// A radius large enough to consume the whole flat edge is capped so the
	// connector tail still has somewhere to attach.
	// halfW = 20, so max = 20 - hintArrowMinHalfBase.
	if got, want := hintBadgeRadius(1000, 40, "bottom"), 20-hintArrowMinHalfBase; got != want {
		t.Errorf("oversized radius = %d, want capped to %d", got, want)
	}

	// A normal radius that already leaves room is left untouched.
	if got := hintBadgeRadius(6, 40, "top"); got != 6 {
		t.Errorf("normal radius should be unchanged, got %d", got)
	}
}

func TestHintBadgePlacement_FullPillKeepsTail(t *testing.T) {
	// Regression: a full-pill border radius on a top/bottom hint must still
	// produce a visible tail once the radius is capped to reserve a flat edge.
	// Previously the tail base expanded to the full badge width and both native
	// renderers collapsed it to a point, dropping the arrow.
	target := image.Pt(200, 150)
	badgeWidth, badgeHeight := 40, 24

	radius := hintBadgeRadius(1000, badgeWidth, "bottom")
	badge, arrow, hasArrow := hintBadgePlacement(target, badgeWidth, badgeHeight, radius, "bottom")

	if !hasArrow {
		t.Fatal("full-pill bottom placement should still draw a tail")
	}

	if arrow.baseLeft.X >= arrow.baseRight.X {
		t.Errorf(
			"tail base collapsed to a point: baseLeft=%v baseRight=%v",
			arrow.baseLeft,
			arrow.baseRight,
		)
	}
	// The base must stay within the flat span the (capped) radius leaves, so the
	// native renderer does not clamp it away.
	flatLeft := badge.Min.X + radius

	flatRight := badge.Max.X - radius
	if arrow.baseLeft.X < flatLeft || arrow.baseRight.X > flatRight {
		t.Errorf("tail base %v..%v escapes flat span [%d,%d]",
			arrow.baseLeft, arrow.baseRight, flatLeft, flatRight)
	}
}

func assertArrowSymmetry(t *testing.T, arrow hintArrowTriangle, centerX int) {
	t.Helper()

	leftGap := centerX - arrow.baseLeft.X

	rightGap := arrow.baseRight.X - centerX
	if leftGap != rightGap {
		t.Errorf("arrow base not symmetric about center: left=%d right=%d", leftGap, rightGap)
	}

	if leftGap <= 0 {
		t.Errorf("arrow base has non-positive width: half-base=%d", leftGap)
	}
}
