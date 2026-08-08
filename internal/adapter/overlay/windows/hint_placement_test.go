//go:build windows

package windows

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// What this backend decides about hint placement on its own: which offset each
// configured placement resolves to, and how the connector arrow is outset to
// give its slanted edges a border. The placement geometry itself is shared with
// the Linux backend and tested beside it (render/badge).
//
// There is no manager-level draw test here, unlike the Linux one: reaching
// DrawHintsWithStyle's placement branch means getting past
// ensureWinOverlayLocked, which creates a real layered window.

// TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary pins this overlay to
// config's declaration of the vocabulary: every value `hints.ui.placement`
// accepts has to resolve to an offset here. A placement added to
// config.HintPlacements() without a branch below fails this test rather than
// drawing a badge in a placement nobody chose.
func TestResolveHintBadgeOffset_EveryPlacementInTheVocabulary(t *testing.T) {
	t.Parallel()

	want := map[string]badge.HintOffset{
		config.HintPlacementTop:    badge.HintAbove,
		config.HintPlacementCenter: badge.HintOnTarget,
		config.HintPlacementBottom: badge.HintBelow,
	}

	for _, placement := range config.HintPlacements() {
		t.Run(placement, func(t *testing.T) {
			t.Parallel()

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
// offset the declared default resolves to, matching what the Linux and macOS
// renderers answer it. A style can reach an overlay before a configuration
// settles it, and a hint that draws on macOS and vanishes here would be this
// package disagreeing with that one.
func TestResolveHintBadgeOffset_UnsetDrawsTheDefault(t *testing.T) {
	t.Parallel()

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
// a value outside the vocabulary is refused, not drawn. Silently drawing it at
// some placement is exactly what this backend used to do with every value —
// the gap this closes.
func TestResolveHintBadgeOffset_UnknownPlacementIsNotSupported(t *testing.T) {
	t.Parallel()

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

// TestOutsetHintArrow_GrowsAwayFromTheBadge pins the border triangle drawn
// under the arrow: it has to be strictly larger on the two slanted edges and
// pushed back into the badge at the base, so the arrow drawn over it leaves an
// even border and nothing pokes out past the badge edge.
func TestOutsetHintArrow_GrowsAwayFromTheBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		offset badge.HintOffset
		// toward is the sign of the direction the arrow points, spelled out
		// rather than re-derived: deriving it here would be the production
		// inference asserting itself.
		toward int
	}{
		{
			name:   "badge above its target, arrow points down",
			offset: badge.HintAbove,
			toward: 1,
		},
		{
			name:   "badge below its target, arrow points up",
			offset: badge.HintBelow,
			toward: -1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, arrow, hasArrow := badge.PlaceHint(
				image.Pt(200, 150), 40, 20, 4, testCase.offset,
			)
			if !hasArrow {
				t.Fatal("expected an arrow to outset")
			}

			outset := outsetHintArrow(arrow, 1)

			if outset.BaseLeft.X >= arrow.BaseLeft.X ||
				outset.BaseRight.X <= arrow.BaseRight.X {
				t.Errorf("base did not widen: %+v from %+v", outset, arrow)
			}

			// The tip moves further from the badge and the base moves back
			// into it, both along the direction the arrow points.
			if (outset.Tip.Y-arrow.Tip.Y)*testCase.toward <= 0 {
				t.Errorf("tip %v did not move away from the badge (was %v)", outset.Tip, arrow.Tip)
			}

			if (outset.BaseLeft.Y-arrow.BaseLeft.Y)*testCase.toward >= 0 {
				t.Errorf(
					"base %v did not move back into the badge (was %v)",
					outset.BaseLeft,
					arrow.BaseLeft,
				)
			}
		})
	}
}
