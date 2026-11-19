package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Unit tests for the hint badge/arrow placement math every overlay that draws a
// hint badge shares. They live beside the math rather than in a backend so they
// run on every CI leg. The Windows backend that also calls it is behind a build
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

	// Center placement is never capped, because the arrow doesn't exist there.
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

// TestResolveHintOffset_EveryPlacementInTheVocabulary pins the Linux and
// Windows overlays to config's declaration of the vocabulary. Every value
// `hints.ui.placement` accepts has to resolve to an offset. A placement added
// to config.HintPlacements() without a branch fails this test rather than
// drawing a badge in a placement nobody chose.
func TestResolveHintOffset_EveryPlacementInTheVocabulary(t *testing.T) {
	t.Parallel()

	want := map[string]badge.HintOffset{
		config.HintPlacementTop:    badge.HintAbove,
		config.HintPlacementCenter: badge.HintOnTarget,
		config.HintPlacementBottom: badge.HintBelow,
	}

	for _, placement := range config.HintPlacements() {
		t.Run(placement, func(t *testing.T) {
			t.Parallel()

			offset, err := badge.ResolveHintOffset(placement)
			if err != nil {
				t.Fatalf("ResolveHintOffset(%q) error = %v, want an offset", placement, err)
			}

			if offset != want[placement] {
				t.Errorf("ResolveHintOffset(%q) = %v, want %v", placement, offset, want[placement])
			}
		})
	}

	if len(want) != len(config.HintPlacements()) {
		t.Fatalf(
			"the vocabulary has %d values but this test maps %d; every one needs an expected offset",
			len(config.HintPlacements()),
			len(want),
		)
	}
}

// TestResolveHintOffset_UnsetDrawsTheDefault pins the empty string to the
// offset the declared default resolves to, matching what the macOS renderer
// answers it (TestHintPlacementValue_UnsetDrawsTheDefault). A style can reach
// an overlay before a configuration settles it, and a hint that draws on macOS
// and vanishes on Linux or Windows would be the backends disagreeing.
func TestResolveHintOffset_UnsetDrawsTheDefault(t *testing.T) {
	t.Parallel()

	unset, err := badge.ResolveHintOffset("")
	if err != nil {
		t.Fatalf(`ResolveHintOffset("") error = %v, want the default's offset`, err)
	}

	fallback, err := badge.ResolveHintOffset(config.HintPlacementDefault)
	if err != nil {
		t.Fatalf("ResolveHintOffset(%q) error = %v", config.HintPlacementDefault, err)
	}

	if unset != fallback {
		t.Errorf(`ResolveHintOffset("") = %v, want %v (the offset of %q)`,
			unset, fallback, config.HintPlacementDefault)
	}
}

// TestResolveHintOffset_UnknownPlacementIsNotSupported pins that a value
// outside the vocabulary is refused, not drawn. Linux used to draw it
// centered and Windows at some placement, so a placement with no branch looked
// like a working one.
func TestResolveHintOffset_UnknownPlacementIsNotSupported(t *testing.T) {
	t.Parallel()

	offset, err := badge.ResolveHintOffset("floating")
	if err == nil {
		t.Fatalf("ResolveHintOffset(%q) = %v, nil; an unrecognized placement must be refused",
			"floating", offset)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("ResolveHintOffset returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}
