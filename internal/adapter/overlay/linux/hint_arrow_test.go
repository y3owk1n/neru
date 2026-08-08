//go:build linux

package linux

import (
	"image"
	"testing"

	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Unit tests for the pure hint badge/arrow placement math shared by the X11 and
// Wayland overlays. Native Cairo rendering is covered by overlay integration
// tests; here we only assert the geometry the C draw calls receive.
func TestHintBadgePlacement_CenterHasNoArrow(t *testing.T) {
	target := image.Pt(100, 100)
	badge, arrow, hasArrow := hintBadgePlacement(target, 40, 20, 4, hintBadgeOnTarget)

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

// TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary pins this overlay to
// config's declaration of the vocabulary: every value `hints.ui.placement`
// accepts has to resolve to an offset here. A placement added to
// config.HintPlacements() without a branch below fails this test rather than
// drawing a badge in a placement nobody chose.
func TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary(t *testing.T) {
	want := map[string]hintBadgeOffset{
		config.HintPlacementTop:    hintBadgeAbove,
		config.HintPlacementCenter: hintBadgeOnTarget,
		config.HintPlacementBottom: hintBadgeBelow,
	}

	for _, placement := range config.HintPlacements() {
		t.Run(placement, func(t *testing.T) {
			offset, err := resolveHintBadgeOffset(placement)
			if err != nil {
				t.Fatalf("resolveHintBadgeOffset(%q) error = %v, want an offset", placement, err)
			}

			if offset != want[placement] {
				t.Errorf(
					"resolveHintBadgeOffset(%q) = %v, want %v",
					placement,
					offset,
					want[placement],
				)
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

// TestResolveHintBadgeOffset_UnsetDrawsTheDefault pins the empty string to the
// offset the declared default resolves to, matching what the macOS renderer
// answers it (TestHintPlacementValue_UnsetDrawsTheDefault). A style can reach
// an overlay before a configuration settles it, and a hint that draws on macOS
// and vanishes on Linux would be this package disagreeing with that one.
func TestResolveHintBadgeOffset_UnsetDrawsTheDefault(t *testing.T) {
	unset, err := resolveHintBadgeOffset("")
	if err != nil {
		t.Fatalf(`resolveHintBadgeOffset("") error = %v, want the default's offset`, err)
	}

	fallback, err := resolveHintBadgeOffset(config.HintPlacementDefault)
	if err != nil {
		t.Fatalf("resolveHintBadgeOffset(%q) error = %v", config.HintPlacementDefault, err)
	}

	if unset != fallback {
		t.Errorf(`resolveHintBadgeOffset("") = %v, want %v (the offset of %q)`,
			unset, fallback, config.HintPlacementDefault)
	}
}

// TestResolveHintBadgeOffset_UnknownPlacementIsNotSupported pins the other half:
// a value outside the vocabulary is refused, not drawn. It used to be drawn
// centered, which made a placement with no Linux branch indistinguishable from
// `center` — silent everywhere and wrong on screen.
func TestResolveHintBadgeOffset_UnknownPlacementIsNotSupported(t *testing.T) {
	offset, err := resolveHintBadgeOffset("floating")
	if err == nil {
		t.Fatalf("resolveHintBadgeOffset(%q) = %v, nil; an unrecognized placement must be refused",
			"floating", offset)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("resolveHintBadgeOffset returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}

// TestLinuxOverlayManager_UnknownHintPlacementIsRefusedNotDrawn pins where that
// refusal reaches a caller: the draw call the mode handler makes, before a
// single badge is painted. A backend is attached here on purpose — with none,
// every draw already reports CodeNotSupported and the assertion would prove
// nothing about the placement.
func TestLinuxOverlayManager_UnknownHintPlacementIsRefusedNotDrawn(t *testing.T) {
	mgr := &Manager{x11: &x11Overlay{}}

	cfg := config.DefaultConfig()
	cfg.Hints.UI.Placement = "floating"

	err := mgr.DrawHintsWithStyle(nil, hintscomponent.BuildStyle(cfg.Hints, nil))
	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"DrawHintsWithStyle with an unrecognized placement returned %v (code %q), "+
				"want CodeNotSupported: the hints would be drawn centered instead",
			err, derrors.GetCode(err),
		)
	}

	// The vocabulary's own values still draw. A guard that refused everything
	// would pass the assertion above and blank the hints overlay.
	for _, placement := range config.HintPlacements() {
		cfg.Hints.UI.Placement = placement

		drawErr := mgr.DrawHintsWithStyle(nil, hintscomponent.BuildStyle(cfg.Hints, nil))
		if drawErr != nil {
			t.Errorf("DrawHintsWithStyle(%q) = %v, want nil", placement, drawErr)
		}
	}
}

func TestHintBadgePlacement_Bottom(t *testing.T) {
	target := image.Pt(200, 150)
	badgeWidth, badgeHeight, radius := 40, 20, 4
	badge, arrow, hasArrow := hintBadgePlacement(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		hintBadgeBelow,
	)

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
	badge, arrow, hasArrow := hintBadgePlacement(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		hintBadgeAbove,
	)

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

	badge, arrow, hasArrow := hintBadgePlacement(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		hintBadgeBelow,
	)
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

func TestHintBadgePlacement_OddBadgeSizeKeepsItsSize(t *testing.T) {
	target := image.Pt(100, 100)

	centered, _, _ := hintBadgePlacement(target, 41, 21, 4, hintBadgeOnTarget)
	if want := image.Rect(80, 90, 121, 111); centered != want {
		t.Errorf("center placement badge = %v, want %v", centered, want)
	}

	below, _, _ := hintBadgePlacement(target, 41, 21, 4, hintBadgeBelow)
	if want := image.Rect(80, 106, 121, 127); below != want {
		t.Errorf("bottom placement badge = %v, want %v", below, want)
	}
}

func TestHintTailEdge(t *testing.T) {
	target := image.Pt(200, 150)

	badge, arrow, hasArrow := hintBadgePlacement(target, 40, 20, 4, hintBadgeBelow)
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailTop {
		t.Errorf(
			"bottom placement (arrow points up) should merge the tail into the top edge, got %d",
			edge,
		)
	}

	badge, arrow, hasArrow = hintBadgePlacement(target, 40, 20, 4, hintBadgeAbove)
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailBottom {
		t.Errorf(
			"top placement (arrow points down) should merge the tail into the bottom edge, got %d",
			edge,
		)
	}

	badge, arrow, hasArrow = hintBadgePlacement(target, 40, 20, 4, hintBadgeOnTarget)
	if edge := hintTailEdge(badge, arrow, hasArrow); edge != hintTailNone {
		t.Errorf("center placement should have no tail, got %d", edge)
	}
}

func TestHintBadgeRadius_ReservesTailFlatEdge(t *testing.T) {
	// Center placement is never capped — the tail doesn't exist there.
	if got := hintBadgeRadius(100, 40, hintBadgeOnTarget); got != 100 {
		t.Errorf("center radius should be unchanged, got %d", got)
	}

	// A radius large enough to consume the whole flat edge is capped so the
	// connector tail still has somewhere to attach.
	// halfW = 20, so max = 20 - hintArrowMinHalfBase.
	if got, want := hintBadgeRadius(
		1000,
		40,
		hintBadgeBelow,
	), 20-hintArrowMinHalfBase; got != want {
		t.Errorf("oversized radius = %d, want capped to %d", got, want)
	}

	// A normal radius that already leaves room is left untouched.
	if got := hintBadgeRadius(6, 40, hintBadgeAbove); got != 6 {
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

	radius := hintBadgeRadius(1000, badgeWidth, hintBadgeBelow)
	badge, arrow, hasArrow := hintBadgePlacement(
		target,
		badgeWidth,
		badgeHeight,
		radius,
		hintBadgeBelow,
	)

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
