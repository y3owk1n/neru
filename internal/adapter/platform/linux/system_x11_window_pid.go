//go:build linux

package linux

import (
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// x11WindowPIDResult names the answers reading _NET_WM_PID off the focused
// window can give. It mirrors NeruX11WindowPIDResult in x11_system.h, mapped by
// name in x11WindowPIDQuery, and makes the same split as x11ActiveWindowResult.
// A live window with no pid answered, and one that closed mid-query failed.
type x11WindowPIDResult int

const (
	// x11WindowPIDFound means the property held a pid.
	x11WindowPIDFound x11WindowPIDResult = iota
	// x11WindowPIDAbsent means the window sets no _NET_WM_PID, which EWMH
	// does not require. Older toolkits and remote clients omit it, so this is
	// an ordinary answer about a healthy window.
	x11WindowPIDAbsent
	// x11WindowPIDWindowGone means the request drew a protocol error because
	// the window closed since _NET_ACTIVE_WINDOW was read.
	x11WindowPIDWindowGone
	// x11WindowPIDQueryFailed means XGetWindowProperty itself failed.
	x11WindowPIDQueryFailed
	// x11WindowPIDMalformed means the property exists but is not the 32-bit
	// CARDINAL value EWMH specifies.
	x11WindowPIDMalformed
)

// errNoWindowPID is the cause a focused window with no pid wraps. It is
// deliberately not errNoFocusedWindow, whose advice to focus a window would be
// false here.
var errNoWindowPID = errors.New("the focused window publishes no pid")

// x11WindowPIDError returns the error a caller owes for result, or nil when a
// pid was read. No pid is CodeNotSupported, which callers degrade through. The
// failures are CodeActionFailed, each named because the fix differs.
func x11WindowPIDError(result x11WindowPIDResult) error {
	switch result {
	case x11WindowPIDFound:
		return nil

	case x11WindowPIDAbsent:
		return derrors.Wrap(
			errNoWindowPID,
			derrors.CodeNotSupported,
			"the focused X11 window advertises no _NET_WM_PID; the property is a "+
				"convention EWMH does not require, and a remote client has no pid "+
				"this machine could use",
		)

	case x11WindowPIDWindowGone:
		return derrors.New(
			derrors.CodeActionFailed,
			"reading _NET_WM_PID drew an X11 protocol error; the window "+
				"_NET_ACTIVE_WINDOW named has closed since it was read",
		)

	case x11WindowPIDQueryFailed:
		return derrors.New(
			derrors.CodeActionFailed,
			"XGetWindowProperty failed reading _NET_WM_PID from the active X11 window",
		)

	case x11WindowPIDMalformed:
		return derrors.New(
			derrors.CodeActionFailed,
			"_NET_WM_PID on the active X11 window is malformed; "+
				"EWMH specifies a single 32-bit CARDINAL value",
		)

	default:
		return derrors.Newf(
			derrors.CodeActionFailed,
			"unknown _NET_WM_PID query result %d on X11",
			int(result),
		)
	}
}
