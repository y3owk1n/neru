//go:build linux

package linux

import (
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// x11ActiveWindowResult names the answers reading _NET_ACTIVE_WINDOW can give.
// It mirrors NeruX11ActiveWindowResult in x11_system.h, mapped by name in
// x11ActiveWindowQuery (system_x11_cgo.go), so the numberings may differ.
//
// The first two answered the question and callers degrade through them. The
// last three are failures they surface. Collapsing them made clicking the
// desktop report a broken X server (#1495).
type x11ActiveWindowResult int

const (
	// x11ActiveWindowFound means the property held a real window id.
	x11ActiveWindowFound x11ActiveWindowResult = iota
	// x11ActiveWindowNone means a live EWMH window manager has nothing
	// focused. The property reads None or is absent, since some (openbox)
	// write it only on first focus, so the _NET_SUPPORTING_WM_CHECK handshake
	// decides this.
	x11ActiveWindowNone
	// x11ActiveWindowNoWindowManager means no live EWMH window manager owns
	// this display. An exited one leaves its properties behind, so the
	// _NET_SUPPORTING_WM_CHECK handshake decides this.
	x11ActiveWindowNoWindowManager
	// x11ActiveWindowQueryFailed means XGetWindowProperty itself failed.
	x11ActiveWindowQueryFailed
	// x11ActiveWindowMalformed means the property exists but is not the
	// 32-bit WINDOW value EWMH specifies.
	x11ActiveWindowMalformed
)

// errNoFocusedWindow is the cause every "nothing has focus" answer wraps, so
// callers recognize it without string-matching and doctor can say focusing a
// window fixes it. Both the X11 arm and the wlroots arm
// (waylandNoFocusedAppError) wrap it. A focused window with no pid is
// errNoWindowPID instead.
var errNoFocusedWindow = errors.New("no window has focus")

// x11ActiveWindowQueryError returns the error a caller owes for result, or nil
// when a window was found. An unfocused desktop is CodeNotSupported, which
// callers degrade through, matching the Wayland arm. The failures are
// CodeActionFailed, each named because the fix differs.
func x11ActiveWindowQueryError(result x11ActiveWindowResult) error {
	switch result {
	case x11ActiveWindowFound:
		return nil

	case x11ActiveWindowNone:
		return derrors.Wrap(
			errNoFocusedWindow,
			derrors.CodeNotSupported,
			"_NET_ACTIVE_WINDOW reports no active window on this X11 display",
		)

	case x11ActiveWindowNoWindowManager:
		return derrors.New(
			derrors.CodeActionFailed,
			"the X11 root window's _NET_SUPPORTING_WM_CHECK handshake does not complete; "+
				"no live EWMH-compliant window manager owns this display, so "+
				"_NET_ACTIVE_WINDOW is unanswerable",
		)

	case x11ActiveWindowQueryFailed:
		return derrors.New(
			derrors.CodeActionFailed,
			"XGetWindowProperty failed reading _NET_ACTIVE_WINDOW from the X11 root window",
		)

	case x11ActiveWindowMalformed:
		return derrors.New(
			derrors.CodeActionFailed,
			"_NET_ACTIVE_WINDOW on the X11 root window is malformed; "+
				"EWMH specifies a single 32-bit WINDOW value",
		)

	default:
		return derrors.Newf(
			derrors.CodeActionFailed,
			"unknown _NET_ACTIVE_WINDOW query result %d on X11",
			int(result),
		)
	}
}
