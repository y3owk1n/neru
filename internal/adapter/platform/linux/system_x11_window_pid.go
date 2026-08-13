//go:build linux

package linux

import (
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// x11WindowPIDResult names the answers reading _NET_WM_PID off the focused
// window can give. It mirrors NeruX11WindowPIDResult in x11_system.h; the
// translation between the two is the switch in x11WindowPIDQuery
// (system_x11_cgo.go), which maps each C constant by name, so the numbering here
// is this package's own and need not match the header's.
//
// It is the same split x11ActiveWindowResult makes one property up, for the same
// reason: a window that is alive and advertises no pid answered the question,
// and callers degrade through it, while a window that closed under the query is
// a failure they surface.
type x11WindowPIDResult int

const (
	// x11WindowPIDFound means the property held a pid.
	x11WindowPIDFound x11WindowPIDResult = iota
	// x11WindowPIDAbsent means the read succeeded and the window sets no
	// _NET_WM_PID. EWMH makes the property a convention rather than a
	// requirement — older toolkits omit it, and a client on another machine
	// has no pid this one could use — so this is an ordinary answer about a
	// perfectly healthy window.
	x11WindowPIDAbsent
	// x11WindowPIDWindowGone means the request drew a protocol error: the
	// window id _NET_ACTIVE_WINDOW named has closed since it was read.
	x11WindowPIDWindowGone
	// x11WindowPIDQueryFailed means XGetWindowProperty itself failed.
	x11WindowPIDQueryFailed
	// x11WindowPIDMalformed means the property exists but is not the 32-bit
	// CARDINAL value EWMH specifies.
	x11WindowPIDMalformed
)

// errNoWindowPID is the cause the "a window has focus, and it publishes no pid"
// error wraps. It is errNoFocusedWindow's sibling and deliberately not the same
// sentinel: both are queries that worked and answered no, but the fix differs,
// and the sentence errNoFocusedWindow buys ("answers as soon as a window takes
// focus") would be false about a session where a window is focused already.
var errNoWindowPID = errors.New("the focused window publishes no pid")

// x11WindowPIDError returns the error a caller owes for result, or nil when a
// pid was read.
//
// A window that advertises no pid is CodeNotSupported, which is the code callers
// degrade through via derrors.IsNotSupported, matching how an unfocused desktop
// is reported one property up. The three failures are CodeActionFailed and each
// says which one it was, because the fix differs: re-sample after the focus
// change that outran the query, look at a broken display server, or file a bug
// against whatever wrote a non-conforming property.
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
