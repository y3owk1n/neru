package recursivegrid

import "image"

// SelectionCallbacks is what a recursive-grid selection tells its owner: the
// selection moved, and the selection resolved. Both carry the point the
// selection now names, and either may be nil when the owner has nothing to do
// with that event.
//
// They travel together as one value rather than as two adjacent func(image.Point)
// parameters because they are identically typed and semantically opposite, so a
// transposed pair compiled and failed nothing (#1346) — the same hazard the row
// and column counts in the same constructor had before GridDimensions closed it
// (#1313). Carrying them together means there is no longer a pair to put in the
// wrong order, and each callback is named at the point it is written down.
type SelectionCallbacks struct {
	// OnUpdate fires every time the selection is refined without resolving:
	// a cell chosen at a depth that can still divide, or a directional move
	// within the current depth. It is the overlay's cue to redraw.
	OnUpdate func(image.Point)
	// OnComplete fires once, when the selection resolves — the keystroke at
	// the final depth, or a zoom that runs out of divisions. Nothing is
	// redrawn afterwards; the point it carries is the answer.
	OnComplete func(image.Point)
}
