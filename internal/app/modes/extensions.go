package modes

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain"
)

// This file holds the optional extensions to Mode: the behavior only some
// modes have, expressed as a narrow unexported interface per axis rather than
// as a switch over domain.Mode with empty arms.
//
// The rules — the assertion each implementer owes, absence semantics, and the
// matrix test that replaces what the exhaustive linter gives a mode switch —
// are in the modes area guide (AGENTS.md). Read it before adding an axis.

// selectionTracker is an optional Mode extension: a mode that remembers where
// its selection sits, separately from where the real cursor is.
//
// Grid and recursive grid carry a selection point; hints, scroll and monitor
// select have nothing of the kind, and a mode that does not implement this is
// silently absent from every call site — the same nothing the empty switch
// arms used to say.
type selectionTracker interface {
	// SelectionPoint reports the mode's current selection in global screen
	// coordinates, and false when the mode has no session or nothing is
	// selected in it.
	SelectionPoint() (image.Point, bool)

	// ClearSelectionPoint forgets the selection and brings the mode's virtual
	// pointer back in step with it. It reports false when there is no session
	// to clear.
	ClearSelectionPoint() bool

	// SelectionAnchor reports the point an indicator should be anchored to
	// instead of the cursor: the selection, whenever the real cursor is not
	// already following it. False means "anchor to the cursor".
	//
	// It must stay a pure read. It runs on the indicator poll tick, where the
	// planIndicatorTick/drawIndicators split exists precisely to keep draws
	// off h.mu, so an implementation that reached the overlay would put one
	// back on it.
	SelectionAnchor() (image.Point, bool)
}

// cellNavigator is an optional Mode extension: a mode whose selection sits in
// a cell of a layout it can slide through without changing the active layer.
type cellNavigator interface {
	// MoveCell slides the selection count cells in dir. A move that would
	// leave the screen is the mode's own business to refuse.
	MoveCell(dir domain.Direction, count int)
}

// activeModeExtension resolves the active mode to the optional extension T,
// reporting false when no mode is active or when the active one does not carry
// that extension. It is the one place the comma-ok assertion is written, so a
// call site reads as "the mode that tracks a selection, if there is one".
//
// T is meant to be one of the extension interfaces above; instantiating it
// with anything else compiles and then matches nothing. It reads the mode map
// newModes builds, so a handler assembled without one reports every extension
// absent.
func activeModeExtension[T any](h *handlerState) (T, bool) {
	var absent T

	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return absent, false
	}

	extension, ok := mode.(T)
	if !ok {
		return absent, false
	}

	return extension, true
}
