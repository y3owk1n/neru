//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/derrors"

// x11ShouldReadWindowGeometry reports whether a focused-window-bounds caller
// should go on to read the active window's geometry, and the error it owes when
// it should not.
//
// Bounds have one answer more than the focused-app path: an unfocused desktop is
// found=false with a *nil* error, because widening to the active screen is the
// right thing to do and nothing went wrong (ports.SystemPort.FocusedWindowBounds
// specifies that). Every other non-answer is the error x11ActiveWindowQueryError
// already writes, so a caller that widens to the whole screen can tell that it
// is guessing — the distinction #1495 made for the pid and #1512 made for the
// compositor CLIs, on the X11 geometry path.
func x11ShouldReadWindowGeometry(result x11ActiveWindowResult) (bool, error) {
	if result == x11ActiveWindowNone {
		return false, nil
	}

	err := x11ActiveWindowQueryError(result)
	if err != nil {
		return false, err
	}

	return true, nil
}

// x11WindowGeometryError is what a caller owes when the active window was found
// and the X server would not describe it.
//
// It is a failure rather than "no bounds": the window existed a request ago, so
// either it closed mid-query or the display server refused two requests it
// should have answered, and both are things a caller widening to the whole
// screen should be able to see it is guessing through.
func x11WindowGeometryError() error {
	return derrors.New(
		derrors.CodeActionFailed,
		"failed to read the geometry of the active X11 window; XGetWindowAttributes "+
			"or XTranslateCoordinates did not answer, or the window closed mid-query",
	)
}
