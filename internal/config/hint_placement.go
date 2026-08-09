package config

// The values `hints.ui.placement` accepts: where a hint badge sits relative to
// the element it labels.
//
// This is the one declaration of that vocabulary. It was written out four
// times — the default, the validator, the macOS hint renderer and the Linux
// overlay each spelled the three strings for itself — and a spelling missed in
// one of them is a placement that validates and then does not draw. It lives
// here rather than in the domain because no domain code places a badge: the
// callers are this package and the overlay backends that draw one, and this
// package is the lowest layer all of them already reach (ADR 0007,
// docs/adr/0007-a-shared-derivation-has-one-implementation.md).
//
// The values cross into Objective-C as well, where the macOS overlay compares
// them as an enum. Go cannot be the implementation of that copy, so
// internal/architecture/hint_placement_vocabulary_test.go pins it to this
// declaration instead.
const (
	// HintPlacementTop draws the badge above its element, with a connector
	// arrow pointing down at it.
	HintPlacementTop = "top"

	// HintPlacementCenter draws the badge over the middle of its element, with
	// no connector arrow.
	HintPlacementCenter = "center"

	// HintPlacementBottom draws the badge below its element, with a connector
	// arrow pointing up at it. This is the default.
	HintPlacementBottom = "bottom"
)

// HintPlacementDefault is the placement a configuration that does not name one
// is settled to.
const HintPlacementDefault = HintPlacementBottom

// HintPlacements returns every accepted `hints.ui.placement` value, in the
// order docs/CONFIGURATION.md lists them. Callers get a fresh slice, so a
// consumer that sorts or filters cannot reach the vocabulary itself.
func HintPlacements() []string {
	return []string{HintPlacementTop, HintPlacementCenter, HintPlacementBottom}
}
