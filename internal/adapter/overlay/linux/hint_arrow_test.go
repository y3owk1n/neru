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

// What this overlay decides about hint placement on its own: where a refused
// placement reaches a caller, and which badge edge the connector tail is
// merged into by the C renderer. Which offset each placement resolves to, and
// the geometry it selects, are shared with the Windows backend and tested
// beside them (render/badge).

// TestLinuxOverlayManager_UnknownHintPlacementIsRefusedNotDrawn pins where a
// placement refusal reaches a caller: the draw call the mode handler makes, before a
// single badge is painted. A backend is attached here on purpose. With none,
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
