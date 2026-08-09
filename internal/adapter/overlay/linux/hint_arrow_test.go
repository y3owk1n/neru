//go:build linux

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// What this overlay decides about hint placement on its own: which offset each
// configured placement resolves to, and which badge edge the connector tail is
// merged into by the C renderer. The geometry itself is shared with the Windows
// backend and tested beside it (render/badge).

// TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary pins this overlay to
// config's declaration of the vocabulary: every value `hints.ui.placement`
// accepts has to resolve to an offset here. A placement added to
// config.HintPlacements() without a branch below fails this test rather than
// drawing a badge in a placement nobody chose.
func TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary(t *testing.T) {
	want := map[string]badge.HintOffset{
		config.HintPlacementTop:    badge.HintAbove,
		config.HintPlacementCenter: badge.HintOnTarget,
		config.HintPlacementBottom: badge.HintBelow,
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

func TestHintTailEdge(t *testing.T) {
	target := image.Pt(200, 150)

	badgeRect, arrow, hasArrow := badge.PlaceHint(target, 40, 20, 4, badge.HintBelow)
	if edge := hintTailEdge(badgeRect, arrow, hasArrow); edge != hintTailTop {
		t.Errorf(
			"bottom placement (arrow points up) should merge the tail into the top edge, got %d",
			edge,
		)
	}

	badgeRect, arrow, hasArrow = badge.PlaceHint(target, 40, 20, 4, badge.HintAbove)
	if edge := hintTailEdge(badgeRect, arrow, hasArrow); edge != hintTailBottom {
		t.Errorf(
			"top placement (arrow points down) should merge the tail into the bottom edge, got %d",
			edge,
		)
	}

	badgeRect, arrow, hasArrow = badge.PlaceHint(target, 40, 20, 4, badge.HintOnTarget)
	if edge := hintTailEdge(badgeRect, arrow, hasArrow); edge != hintTailNone {
		t.Errorf("center placement should have no tail, got %d", edge)
	}
}
