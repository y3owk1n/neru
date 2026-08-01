package recursivegrid

import (
	"image"
	"slices"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// gridState captures everything MoveDirection mutates, so a step that cannot
// be completed can be rolled back and leave the grid exactly as it was.
type gridState struct {
	currentBounds image.Rectangle
	history       []image.Rectangle
	depth         int
	finalCell     Cell
	hasFinalCell  bool
}

// snapshot copies the mutable navigation state. history is cloned because
// Reset truncates the backing array in place.
func (qg *RecursiveGrid) snapshot() gridState {
	return gridState{
		currentBounds: qg.currentBounds,
		history:       slices.Clone(qg.history),
		depth:         qg.depth,
		finalCell:     qg.finalCell,
		hasFinalCell:  qg.hasFinalCell,
	}
}

// restore puts back a snapshot taken by snapshot.
func (qg *RecursiveGrid) restore(state gridState) {
	qg.currentBounds = state.currentBounds
	qg.history = state.history
	qg.depth = state.depth
	qg.finalCell = state.finalCell
	qg.hasFinalCell = state.hasFinalCell
}

// MoveDirection slides the current selection one cell in dir, repeated count
// times, without changing the active depth. Movement is spatial rather than
// confined to the parent's cells: the selection crosses into a neighboring
// parent when it reaches the edge of its own, and the ancestor chain is
// rebuilt to match, so backtracking afterwards stays correct.
//
// A step that would leave the initial bounds is refused, so movement stops at
// the screen edge instead of wrapping. Counts below one are treated as one.
//
// Returns the resulting selection center and whether anything moved.
func (qg *RecursiveGrid) MoveDirection(dir domain.Direction, count int) (image.Point, bool) {
	count = max(count, 1)

	moved := false

	for range count {
		if !qg.moveOnce(dir) {
			break
		}

		moved = true
	}

	return qg.SelectionCenter(), moved
}

// moveOnce slides the selection by exactly one cell. It reports whether the
// move happened; when it does not, the grid is left untouched.
func (qg *RecursiveGrid) moveOnce(dir domain.Direction) bool {
	rect := qg.EffectiveBounds()
	if rect.Empty() {
		return false
	}

	deltaX, deltaY := dir.Delta()
	next := rect.Add(image.Point{X: deltaX * rect.Dx(), Y: deltaY * rect.Dy()})

	// rectCenter keeps the probe inside next, which matters for one-pixel
	// cells: an unclamped center would resolve to the cell beyond the one we
	// mean, skipping a cell and drifting diagonally.
	target := rectCenter(next)

	// Stop at the screen edge. At depth 0 with no final cell the selection is
	// the whole screen, so every direction lands outside and this is a no-op.
	if !target.In(qg.initialBounds) {
		return false
	}

	previous := qg.snapshot()

	// Re-walk from the root towards the target instead of patching bounds in
	// place: ZoomToPoint resolves the containing cell at every depth using
	// that depth's own layout, so per-depth overrides and remainder-sized
	// cells come out right and the rebuilt history matches the new position.
	qg.Reset()
	qg.ZoomToPoint(target, previous.depth)

	// Cells within a level can differ by a pixel, so a path that reached this
	// depth before may bottom out one level early. Rather than silently
	// landing at the wrong depth, refuse the move.
	if qg.depth != previous.depth {
		qg.restore(previous)

		return false
	}

	if previous.hasFinalCell {
		cell := qg.CellForPoint(target)
		if cell < 0 {
			qg.restore(previous)

			return false
		}

		qg.finalCell = cell
		qg.hasFinalCell = true
	}

	return true
}
