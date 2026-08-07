//go:build darwin

package hints

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestHintPlacementValue_EveryPlacementMapsToItsOwnConstant is the macOS half
// of the placement pin. The architecture test keeps the C header and the
// Objective-C enum agreeing about the numbers; this keeps the translation
// honest, because an unrecognized placement falls through to the default here
// rather than failing. A placement added to config.HintPlacements() and not to
// this switch would validate, reach the overlay and draw at the bottom.
func TestHintPlacementValue_EveryPlacementMapsToItsOwnConstant(t *testing.T) {
	t.Parallel()

	byValue := make(map[int]string)

	for _, placement := range config.HintPlacements() {
		value := hintPlacementValue(placement)

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
// documented default says.
func TestHintPlacementValue_UnsetDrawsTheDefault(t *testing.T) {
	t.Parallel()

	unset := hintPlacementValue("")

	fallback := hintPlacementValue(config.HintPlacementDefault)
	if unset != fallback {
		t.Errorf(
			"hintPlacementValue(\"\") = %d, want %d (the value of %q)",
			unset, fallback, config.HintPlacementDefault,
		)
	}
}
