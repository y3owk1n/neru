//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/derrors"

// x11ShouldReadWindowGeometry reports whether a focused-window-bounds caller
// should go on to read the active window's geometry, and the error it owes when
// it should not. An unfocused desktop is found=false with a nil error
// (ports.SystemPort.FocusedWindowBounds). Every other non-answer is an error,
// so a caller widening to the screen knows it is guessing (#1495, #1512).
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
// and the X server would not describe it. It is a failure, not "no bounds".
func x11WindowGeometryError() error {
	return derrors.New(
		derrors.CodeActionFailed,
		"failed to read the geometry of the active X11 window; XGetWindowAttributes "+
			"or XTranslateCoordinates did not answer, or the window closed mid-query",
	)
}
