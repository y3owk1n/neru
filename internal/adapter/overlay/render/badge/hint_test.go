package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

// Unit tests for the hint badge/arrow placement math every overlay that draws a
// hint badge shares. They live beside the math rather than in a backend so they
// run on every CI leg: the Windows backend that also calls it is behind a build
// tag no test run here can reach.

func TestPlaceHint_CenterHasNoArrow(t *testing.T) {
	t.Parallel()

	target := image.Pt(100, 100)
	rect, arrow, hasArrow := badge.PlaceHint(target, 40, 20, 4, badge.HintOnTarget)

	if hasArrow {
		t.Fatalf("center placement should not draw an arrow, got %+v", arrow)
	}
	// Center placement keeps the badge centered on the target.
	if got := rect.Min.X + rect.Dx()/2; got != target.X {
		t.Errorf("badge center X = %d, want %d", got, target.X)
	}

	if got := rect.Min.Y + rect.Dy()/2; got != target.Y {
		t.Errorf("badge center Y = %d, want %d", got, target.Y)
	}
}

func TestPlaceHint_Bottom(t *testing.T) {
	t.Parallel()

	target := image.Pt(200, 150)
	badgeWidth, badgeHeight, radius := 40, 20, 4
	rect, arrow, hasArrow := badge.PlaceHint(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		badge.HintBelow,
	)

	if !hasArrow {
		t.Fatal("bottom placement should draw an arrow")
	}

	// Badge sits below the target, offset by gap + arrow height + half height.
	wantTop := target.Y + badge.HintGap + badge.HintArrowHeight
	if rect.Min.Y != wantTop {
		t.Errorf("badge top = %d, want %d", rect.Min.Y, wantTop)
	}

	if got := rect.Min.X + rect.Dx()/2; got != target.X {
		t.Errorf(
			"badge is not horizontally centered on target: center X = %d, want %d",
			got,
			target.X,
		)
	}

	// The arrow base is flush with the badge top edge and the tip points up at
	// the target, landing one gap short of it.
	if arrow.BaseLeft.Y != rect.Min.Y || arrow.BaseRight.Y != rect.Min.Y {
		t.Errorf("arrow base not on badge top edge: baseLeft=%v baseRight=%v top=%d",
			arrow.BaseLeft, arrow.BaseRight, rect.Min.Y)
	}

	if arrow.Tip.Y >= arrow.BaseLeft.Y {
		t.Errorf(
			"bottom-placement arrow tip should point up (above base): tip=%v base=%v",
			arrow.Tip,
			arrow.BaseLeft,
		)
	}

	if arrow.Tip.X != target.X {
		t.Errorf("arrow tip X = %d, want %d (aligned with target)", arrow.Tip.X, target.X)
	}

	if wantTipY := rect.Min.Y - badge.HintArrowHeight; arrow.Tip.Y != wantTipY {
		t.Errorf("arrow tip Y = %d, want %d", arrow.Tip.Y, wantTipY)
	}

	assertArrowSymmetry(t, arrow, target.X)
}

func TestPlaceHint_Top(t *testing.T) {
	t.Parallel()

	target := image.Pt(200, 150)
	badgeWidth, badgeHeight, radius := 40, 20, 4
	rect, arrow, hasArrow := badge.PlaceHint(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		badge.HintAbove,
	)

	if !hasArrow {
		t.Fatal("top placement should draw an arrow")
	}

	// Badge sits above the target.
	wantBottom := target.Y - badge.HintGap - badge.HintArrowHeight
	if rect.Max.Y != wantBottom {
		t.Errorf("badge bottom = %d, want %d", rect.Max.Y, wantBottom)
	}

	// The arrow base is flush with the badge bottom edge and the tip points down
	// at the target.
	if arrow.BaseLeft.Y != rect.Max.Y || arrow.BaseRight.Y != rect.Max.Y {
		t.Errorf("arrow base not on badge bottom edge: baseLeft=%v baseRight=%v bottom=%d",
			arrow.BaseLeft, arrow.BaseRight, rect.Max.Y)
	}

	if arrow.Tip.Y <= arrow.BaseLeft.Y {
		t.Errorf(
			"top-placement arrow tip should point down (below base): tip=%v base=%v",
			arrow.Tip,
			arrow.BaseLeft,
		)
	}

	if wantTipY := rect.Max.Y + badge.HintArrowHeight; arrow.Tip.Y != wantTipY {
		t.Errorf("arrow tip Y = %d, want %d", arrow.Tip.Y, wantTipY)
	}

	assertArrowSymmetry(t, arrow, target.X)
}

// TestPlaceHint_TopAndBottomAreMirrorImages pins the pair a user actually
// compares: `top` and `bottom` put the badge the same distance from the target,
// on opposite sides, with the arrow pointing back at it either way. A backend
// that got one sign wrong would still pass each single-placement test above.
func TestPlaceHint_TopAndBottomAreMirrorImages(t *testing.T) {
	t.Parallel()

	target := image.Pt(200, 150)

	above, aboveArrow, _ := badge.PlaceHint(target, 40, 20, 4, badge.HintAbove)
	below, belowArrow, _ := badge.PlaceHint(target, 40, 20, 4, badge.HintBelow)

	if gapAbove, gapBelow := target.Y-above.Max.Y, below.Min.Y-target.Y; gapAbove != gapBelow {
		t.Errorf("badge sits %d px above the target but %d px below it", gapAbove, gapBelow)
	}

	if aboveArrow.Tip.Y <= target.Y-badge.HintGap-1 {
		t.Errorf(
			"top-placement arrow tip %v does not reach down to target %v",
			aboveArrow.Tip,
			target,
		)
	}

	if belowArrow.Tip.Y >= target.Y+badge.HintGap+1 {
		t.Errorf(
			"bottom-placement arrow tip %v does not reach up to target %v",
			belowArrow.Tip,
			target,
		)
	}
}

func TestPlaceHint_ArrowBaseClampedToFlatEdge(t *testing.T) {
	t.Parallel()

	// A narrow badge with a large radius leaves little flat edge; the arrow base
	// must clamp inside the corner radius rather than run onto the rounded
	// corners or overflow the badge.
	target := image.Pt(50, 50)
	badgeWidth, badgeHeight, radius := 14, 20, 6

	rect, arrow, hasArrow := badge.PlaceHint(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		badge.HintBelow,
	)
	if !hasArrow {
		t.Fatal("expected an arrow")
	}

	halfBase := arrow.BaseRight.X - target.X
	if maxHalf := rect.Dx()/2 - radius; halfBase > maxHalf {
		t.Errorf("arrow half-base %d exceeds flat-edge limit %d", halfBase, maxHalf)
	}

	if arrow.BaseLeft.X < rect.Min.X || arrow.BaseRight.X > rect.Max.X {
		t.Errorf("arrow base %v..%v overflows badge %v", arrow.BaseLeft, arrow.BaseRight, rect)
	}
}

func TestPlaceHint_OddBadgeSizeKeepsItsSize(t *testing.T) {
	t.Parallel()

	target := image.Pt(100, 100)

	centered, _, _ := badge.PlaceHint(target, 41, 21, 4, badge.HintOnTarget)
	if want := image.Rect(80, 90, 121, 111); centered != want {
		t.Errorf("center placement badge = %v, want %v", centered, want)
	}

	below, _, _ := badge.PlaceHint(target, 41, 21, 4, badge.HintBelow)
	if want := image.Rect(80, 106, 121, 127); below != want {
		t.Errorf("bottom placement badge = %v, want %v", below, want)
	}
}

func TestHintRadius_ReservesArrowFlatEdge(t *testing.T) {
	t.Parallel()

	// Center placement is never capped — the arrow doesn't exist there.
	if got := badge.HintRadius(100, 40, badge.HintOnTarget); got != 100 {
		t.Errorf("center radius should be unchanged, got %d", got)
	}

	// A radius large enough to consume the whole flat edge is capped so the
	// connector arrow still has somewhere to attach.
	// halfW = 20, so max = 20 - HintArrowMinHalfBase.
	if got, want := badge.HintRadius(
		1000,
		40,
		badge.HintBelow,
	), 20-badge.HintArrowMinHalfBase; got != want {
		t.Errorf("oversized radius = %d, want capped to %d", got, want)
	}

	// A normal radius that already leaves room is left untouched.
	if got := badge.HintRadius(6, 40, badge.HintAbove); got != 6 {
		t.Errorf("normal radius should be unchanged, got %d", got)
	}
}

func TestPlaceHint_FullPillKeepsArrow(t *testing.T) {
	t.Parallel()

	// Regression: a full-pill border radius on a top/bottom hint must still
	// produce a visible arrow once the radius is capped to reserve a flat edge.
	// Previously the base expanded to the full badge width and both native
	// renderers collapsed it to a point, dropping the arrow.
	target := image.Pt(200, 150)
	badgeWidth, badgeHeight := 40, 24

	radius := badge.HintRadius(1000, badgeWidth, badge.HintBelow)
	rect, arrow, hasArrow := badge.PlaceHint(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		badge.HintBelow,
	)

	if !hasArrow {
		t.Fatal("full-pill bottom placement should still draw an arrow")
	}

	if arrow.BaseLeft.X >= arrow.BaseRight.X {
		t.Errorf(
			"arrow base collapsed to a point: baseLeft=%v baseRight=%v",
			arrow.BaseLeft,
			arrow.BaseRight,
		)
	}
	// The base must stay within the flat span the (capped) radius leaves, so the
	// native renderer does not clamp it away.
	flatLeft := rect.Min.X + radius

	flatRight := rect.Max.X - radius
	if arrow.BaseLeft.X < flatLeft || arrow.BaseRight.X > flatRight {
		t.Errorf("arrow base %v..%v escapes flat span [%d,%d]",
			arrow.BaseLeft, arrow.BaseRight, flatLeft, flatRight)
	}
}

func assertArrowSymmetry(t *testing.T, arrow badge.HintArrow, centerX int) {
	t.Helper()

	leftGap := centerX - arrow.BaseLeft.X

	rightGap := arrow.BaseRight.X - centerX
	if leftGap != rightGap {
		t.Errorf("arrow base not symmetric about center: left=%d right=%d", leftGap, rightGap)
	}

	if leftGap <= 0 {
		t.Errorf("arrow base has non-positive width: half-base=%d", leftGap)
	}
}
