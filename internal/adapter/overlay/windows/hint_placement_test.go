//go:build windows

package windows

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

// What this backend decides about hint placement on its own: how the connector
// arrow is outset to give its slanted edges a border. Which offset each
// placement resolves to, and the geometry it selects, are shared with the
// Linux backend and tested beside them (render/badge).
//
// There is no manager-level draw test here, unlike the Linux one. Reaching
// DrawHintsWithStyle's placement branch means getting past
// ensureWinOverlayLocked, which creates a real layered window.

// TestOutsetHintArrow_GrowsAwayFromTheBadge pins the border triangle drawn
// under the arrow. It has to be strictly larger on the two slanted edges and
// pushed back into the badge at the base, so the arrow drawn over it leaves an
// even border and nothing pokes out past the badge edge.
func TestOutsetHintArrow_GrowsAwayFromTheBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		offset badge.HintOffset
		// toward is the sign of the direction the arrow points, spelled out
		// rather than re-derived. Deriving it here would be the production
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
