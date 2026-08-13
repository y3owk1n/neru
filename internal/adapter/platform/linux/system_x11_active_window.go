//go:build linux

package linux

import (
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// x11ActiveWindowResult names the answers reading _NET_ACTIVE_WINDOW on the X11
// root window can give. It mirrors NeruX11ActiveWindowResult in x11_system.h;
// the translation between the two is the switch in x11ActiveWindowQuery
// (system_x11_cgo.go), which maps each C constant by name, so the numbering here
// is this package's own and need not match the header's.
//
// The distinction that matters is between the first two and the last three: a
// desktop where nothing is focused answered the question, and callers degrade
// through it, while a query that never got an answer is a failure they surface.
// Collapsing all five into "1 or 0" is what made clicking the desktop
// background report a broken X server (#1495).
type x11ActiveWindowResult int

const (
	// x11ActiveWindowFound means the property held a real window id.
	x11ActiveWindowFound x11ActiveWindowResult = iota
	// x11ActiveWindowNone means the query succeeded and a live EWMH window
	// manager has nothing focused — the property reads None, or it is absent
	// because that window manager writes it only once something takes focus
	// (openbox does, which is why the _NET_SUPPORTING_WM_CHECK handshake and
	// not _NET_ACTIVE_WINDOW decides this).
	x11ActiveWindowNone
	// x11ActiveWindowNoWindowManager means no live EWMH window manager owns
	// this display, so nothing will ever answer the question. A window manager
	// that exited leaves its root-window properties behind, so this is decided
	// by the _NET_SUPPORTING_WM_CHECK handshake, which its window cannot
	// complete once it is gone.
	x11ActiveWindowNoWindowManager
	// x11ActiveWindowQueryFailed means XGetWindowProperty itself failed.
	x11ActiveWindowQueryFailed
	// x11ActiveWindowMalformed means the property exists but is not the
	// 32-bit WINDOW value EWMH specifies.
	x11ActiveWindowMalformed
)

// errNoFocusedWindow is the cause every "the display answered, and the answer
// is that nothing has focus" error wraps. It exists so callers can recognize
// that one state without string-matching a message: the capability surface
// explains it differently from a failure, because focusing any window fixes it
// and installing something does not.
//
// Both arms wrap it: the X11 one for a display whose _NET_ACTIVE_WINDOW reads
// None, and the wlroots one for a foreign-toplevel manager with nothing
// activated (waylandNoFocusedAppError). What it does not cover is a window that
// has focus and no pid to report — that is errNoWindowPID, a sibling sentinel,
// because the sentence this one buys would be false there.
var errNoFocusedWindow = errors.New("no window has focus")

// x11ActiveWindowQueryError returns the error a caller owes for result, or nil
// when a window was found.
//
// An unfocused desktop is CodeNotSupported, which is the code callers degrade
// through via derrors.IsNotSupported, and the answer the Wayland arm already
// gives for the same state (waylandFocusedApplicationPID). The three failures
// are CodeActionFailed and each says which one it was, because the fix differs:
// start a window manager, look at a broken display server, or file a bug
// against whatever wrote a non-conforming property.
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
