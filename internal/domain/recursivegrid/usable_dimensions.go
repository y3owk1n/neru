package recursivegrid

import "github.com/y3owk1n/neru/internal/domain"

// The two limits a recursive-grid shape has to clear. They live here because
// UsableDimensions is the only thing that applies them.
const (
	// MinGridDimension is the minimum allowed value for grid columns or rows.
	MinGridDimension = 1
	// MinUsableCellCount is the smallest number of cells a recursive grid can
	// narrow with. One cell is a selection that selects the whole region, so
	// the grid would never bottom out.
	MinUsableCellCount = 2
)

// UsableDimensions answers the shape a recursive grid can actually be drawn and
// navigated with, given the shape it was asked for. It returns the shape to use
// and whether that is the one it was given: a pair with fewer than
// MinGridDimension columns or rows, or fewer than MinUsableCellCount cells,
// cannot narrow anything, so DefaultDimensions() comes back instead and the
// second return is false.
//
// The whole pair is replaced rather than the offending half, because half a
// configured shape is a shape nobody chose and no key mapping is the right
// length for.
//
// This is the one implementation of the *fallback* (ADR 0007). It lived here
// and in the macOS draw, and the two copies had drifted: only one of them
// logged, only one of them reset the key mapping alongside the shape, and the
// macOS copy's comment still named a default the constants had moved off
// (#1345). Callers keep what is genuinely theirs — the manager warns with the
// values it rejected, and the macOS draw reaches for DefaultKeys when this
// reports a replacement, because a key mapping cut for the configured shape
// does not fit the fallback.
//
// Two weaker tests of the same limits are left standing on purpose, and naming
// them is the honest form of that claim:
//
//   - config.ValidateRecursiveGrid states both limits and *refuses* — a shape a
//     user wrote is answered by telling them their configuration is wrong, not
//     by quietly navigating a grid they did not ask for. It is the reason a
//     degenerate shape does not reach a Manager in the first place.
//   - The Linux and Windows draws test only that neither side is zero or less,
//     and return rather than fall back, so they draw nothing for a zero side
//     and one unnarrowable cell for a 1x1. They are not corrected here because
//     that would be a behavior change on two platforms rather than a
//     refactor; what makes it moot is that the two answers above run first, and
//     TestRecursiveGridFrame_DegenerateShapeReachesEveryBackendAsTheDefault
//     (internal/app/modes) pins that no backend is ever handed a shape this
//     would have replaced.
func UsableDimensions(dims domain.GridDimensions) (domain.GridDimensions, bool) {
	if dims.Cols < MinGridDimension || dims.Rows < MinGridDimension ||
		dims.CellCount() < MinUsableCellCount {
		return DefaultDimensions(), false
	}

	return dims, true
}
