//go:build darwin

package hints

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// unknownPlacement is a `hints.ui.placement` value the vocabulary does not
// declare — the shape a fourth placement would have on the day someone adds it
// to config and forgets this renderer.
const unknownPlacement = "floating"

// TestHintPlacementValue_EveryPlacementMapsToItsOwnConstant is the macOS half
// of the placement pin. The architecture test keeps the C header and the
// Objective-C enum agreeing about the numbers; this keeps the translation
// honest. A placement added to config.HintPlacements() and not to this switch
// would validate, reach the overlay and be refused rather than drawn — this
// test is what says so before a user sees it.
func TestHintPlacementValue_EveryPlacementMapsToItsOwnConstant(t *testing.T) {
	t.Parallel()

	byValue := make(map[int]string)

	for _, placement := range config.HintPlacements() {
		value, err := hintPlacementValue(placement)
		if err != nil {
			t.Errorf("hintPlacementValue(%q) error = %v, want a constant", placement, err)

			continue
		}

		clashing, taken := byValue[value]
		if taken {
			t.Errorf(
				"placements %q and %q both draw as C constant %d; "+
					"hintPlacementValue is missing a case",
				clashing, placement, value,
			)
		}

		byValue[value] = placement
	}
}

// TestHintPlacementValue_UnsetDrawsTheDefault pins the empty string to the
// same constant the declared default resolves to, so a configuration that
// reaches the overlay before validation settles it still draws where the
// documented default says. Refusing it instead would leave an unsettled style
// drawing no hints at all, and the Linux overlay answers it the same way
// (resolveHintBadgeOffset).
func TestHintPlacementValue_UnsetDrawsTheDefault(t *testing.T) {
	t.Parallel()

	unset, err := hintPlacementValue("")
	if err != nil {
		t.Fatalf(`hintPlacementValue("") error = %v, want the default's constant`, err)
	}

	fallback, err := hintPlacementValue(config.HintPlacementDefault)
	if err != nil {
		t.Fatalf("hintPlacementValue(%q) error = %v", config.HintPlacementDefault, err)
	}

	if unset != fallback {
		t.Errorf(
			"hintPlacementValue(\"\") = %d, want %d (the value of %q)",
			unset, fallback, config.HintPlacementDefault,
		)
	}
}

// TestHintPlacementValue_UnknownPlacementIsNotSupported pins the other half of
// the vocabulary: a value outside it is refused, not translated. It used to
// come back as HINT_PLACEMENT_BOTTOM, which made a placement with no branch
// here indistinguishable from `bottom` — silent everywhere and wrong on
// screen. The Linux overlay already refuses the same value (#1331).
func TestHintPlacementValue_UnknownPlacementIsNotSupported(t *testing.T) {
	t.Parallel()

	value, err := hintPlacementValue(unknownPlacement)
	if err == nil {
		t.Fatalf(
			"hintPlacementValue(%q) = %d, nil; an unrecognized placement must be refused",
			unknownPlacement, value,
		)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"hintPlacementValue(%q) returned %v (code %q), want CodeNotSupported",
			unknownPlacement, err, derrors.GetCode(err),
		)
	}
}

// placementStyle is a resolved hint style that differs from the next one only
// in where the labels are placed, which is all the draw tests below vary.
func placementStyle(t *testing.T, placement string) StyleMode {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Hints.UI.Placement = placement

	return BuildStyle(cfg.Hints, nil)
}

// drawnHints is one hint to put on screen. The draw refuses an empty set
// before it looks at anything else, so a set with something in it is what the
// placement tests need.
func drawnHints() []*Hint {
	return []*Hint{NewHint("AA", image.Pt(100, 100), image.Pt(20, 20), "")}
}

// newTestOverlay builds a hint renderer over no window. Every C draw call
// guards a nil window and returns, so this exercises the Go side of a draw —
// which is where the placement is resolved — without an NSWindow.
func newTestOverlay(t *testing.T) *Overlay {
	t.Helper()

	overlay, err := NewOverlayWithWindow(config.DefaultConfig().Hints, zap.NewNop(), nil)
	if err != nil {
		t.Fatalf("NewOverlayWithWindow() error = %v", err)
	}

	return overlay
}

// TestOverlay_DrawHintsWithStyle_UnknownPlacementIsRefusedNotDrawn pins where
// the refusal reaches a caller: the draw the mode handler makes, before a
// single label crosses into Objective-C. Drawing them at the bottom instead
// would leave the user looking at hints in a placement they did not choose,
// with nothing anywhere saying so.
func TestOverlay_DrawHintsWithStyle_UnknownPlacementIsRefusedNotDrawn(t *testing.T) {
	t.Parallel()

	err := newTestOverlay(t).DrawHintsWithStyle(drawnHints(), placementStyle(t, unknownPlacement))
	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"DrawHintsWithStyle with an unrecognized placement = %v (code %q), "+
				"want CodeNotSupported: the hints would be drawn at the bottom instead",
			err, derrors.GetCode(err),
		)
	}

	// The vocabulary's own placements still draw. A guard that refused
	// everything would pass the assertion above and blank the hints overlay.
	for _, placement := range config.HintPlacements() {
		drawErr := newTestOverlay(t).DrawHintsWithStyle(drawnHints(), placementStyle(t, placement))
		if drawErr != nil {
			t.Errorf("DrawHintsWithStyle(%q) = %v, want nil", placement, drawErr)
		}
	}
}

// TestOverlay_DrawHintsWithStyle_UnknownPlacementIsRefusedOnARedraw pins the
// refusal on a draw that follows one already on screen — the keystroke case,
// which would otherwise take the incremental route and build its own style for
// the C call. The resolution happens once above both routes, so this asserts
// that placement of the guard rather than a second guard: a refusal put inside
// the full redraw would let every keystroke after the first through.
//
// What no test can reach is which constant each route hands to Objective-C:
// the draw's only effect is the C call. The architecture test pins the
// translation, and this pins that neither route runs without it.
func TestOverlay_DrawHintsWithStyle_UnknownPlacementIsRefusedOnARedraw(t *testing.T) {
	t.Parallel()

	overlay := newTestOverlay(t)

	firstErr := overlay.DrawHintsWithStyle(
		drawnHints(),
		placementStyle(t, config.HintPlacementDefault),
	)
	if firstErr != nil {
		t.Fatalf("DrawHintsWithStyle() error = %v, want the first draw to land", firstErr)
	}

	// A structurally different set, so the redraw cannot be answered by the
	// "nothing changed" shortcut.
	redrawn := append(drawnHints(), NewHint("AB", image.Pt(300, 400), image.Pt(20, 20), ""))

	err := overlay.DrawHintsWithStyle(redrawn, placementStyle(t, unknownPlacement))
	if !derrors.IsNotSupported(err) {
		t.Errorf(
			"redraw with an unrecognized placement = %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err),
		)
	}
}

// TestOverlay_DrawHintsWithStyle_UnknownPlacementStillClearsAnEmptySet pins the
// one draw the refusal does not cover. An empty hint set means "take the labels
// off the screen", and a placement decides where a label goes, not whether one
// can be removed — refusing here would strand whatever is on screen there.
func TestOverlay_DrawHintsWithStyle_UnknownPlacementStillClearsAnEmptySet(t *testing.T) {
	t.Parallel()

	err := newTestOverlay(t).DrawHintsWithStyle(nil, placementStyle(t, unknownPlacement))
	if err != nil {
		t.Errorf("DrawHintsWithStyle(nil) = %v, want nil: an empty set clears the overlay", err)
	}
}
