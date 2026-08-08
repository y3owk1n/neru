package config

// The values `hints.search_input_ui.position` accepts: which corner or edge of
// the screen the hints search box is anchored to.
//
// This is the one declaration of that vocabulary. The seven strings were
// written three times — the validator's accepted set, the message it refuses an
// unknown value with, and the render layer's constants — and a spelling missed
// in one of them is either a position that validates and then draws somewhere
// else, or a message that lies about what is accepted. It lives here rather
// than in the render package because the validator and the renderer both need
// it and this package is the lower of the two (ADR 0007,
// docs/adr/0007-a-shared-derivation-has-one-implementation.md).
const (
	// SearchInputPositionTopLeft anchors the box to the top-left corner, where
	// the configured offsets are its position outright.
	SearchInputPositionTopLeft = "top_left"

	// SearchInputPositionTopCenter centers the box horizontally along the top
	// edge.
	SearchInputPositionTopCenter = "top_center"

	// SearchInputPositionTopRight anchors the box to the top-right corner.
	SearchInputPositionTopRight = "top_right"

	// SearchInputPositionCenter centers the box on both axes.
	SearchInputPositionCenter = "center"

	// SearchInputPositionBottomLeft anchors the box to the bottom-left corner.
	SearchInputPositionBottomLeft = "bottom_left"

	// SearchInputPositionBottomCenter centers the box horizontally along the
	// bottom edge. This is the default.
	SearchInputPositionBottomCenter = "bottom_center"

	// SearchInputPositionBottomRight anchors the box to the bottom-right
	// corner.
	SearchInputPositionBottomRight = "bottom_right"
)

// SearchInputPositionDefault is the anchor a configuration that does not name
// one is shipped with. Unlike `hints.ui.placement`, an unset value is refused
// rather than settled — this is the default written into the defaults, not a
// fallback the validator applies.
const SearchInputPositionDefault = SearchInputPositionBottomCenter

// SearchInputPositions returns every accepted `hints.search_input_ui.position`
// value, in the order docs/CONFIGURATION.md lists them. Callers get a fresh
// slice, so a consumer that sorts or filters cannot reach the vocabulary
// itself.
func SearchInputPositions() []string {
	return []string{
		SearchInputPositionTopLeft,
		SearchInputPositionTopCenter,
		SearchInputPositionTopRight,
		SearchInputPositionCenter,
		SearchInputPositionBottomLeft,
		SearchInputPositionBottomCenter,
		SearchInputPositionBottomRight,
	}
}
