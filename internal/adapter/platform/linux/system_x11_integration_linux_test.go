//go:build integration && linux && cgo

package linux

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// xpropTimeout bounds the independent session probe. A wedged X server must
// fail this test's premise quickly rather than hang the desktop tier.
const xpropTimeout = 5 * time.Second

// TestX11FocusedApplicationPID_AWindowManagerIsNeverMistakenForNoWindowManager
// is the one branch of the _NET_ACTIVE_WINDOW split that only a live X server
// can settle.
//
// A window manager with nothing focused and a display with no window manager
// both leave _NET_ACTIVE_WINDOW absent, and they land on opposite sides of the
// CodeNotSupported / CodeActionFailed line this package draws — so what
// separates them is the _NET_SUPPORTING_WM_CHECK handshake, not
// _NET_ACTIVE_WINDOW and not the presence of _NET_SUPPORTED. Openbox, which
// CI's X11 leg runs, is exactly the case that proves the first half: it
// advertises _NET_ACTIVE_WINDOW and then writes no such property until a window
// takes focus. Getting this backwards reports a healthy desktop as a broken
// one, which is the whole of #1495.
func TestX11FocusedApplicationPID_AWindowManagerIsNeverMistakenForNoWindowManager(
	t *testing.T,
) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X11 display; this asserts a rule about a live root window")
	}

	if !rootHasLiveWindowManager(t) {
		t.Skip("no live EWMH window manager owns this display; nothing to mistake it for")
	}

	_, err := x11FocusedApplicationPID()

	switch {
	case err == nil:
		// A window is focused and exposes its PID — the ordinary path.
	case derrors.IsNotSupported(err):
		// Nothing is focused. That is the answer this ticket exists to allow,
		// and callers degrade through it.
	case strings.Contains(err.Error(), windowPIDProperty):
		// The focused window exposes no _NET_WM_PID. A different property, and
		// not what this test is about.
	default:
		t.Fatalf(
			"a display whose _NET_SUPPORTING_WM_CHECK handshake completes answered %v "+
				"(code %q); under a live window manager the active-window query must not fail",
			err, derrors.GetCode(err),
		)
	}
}

// windowPIDProperty names the second property the focused-app path reads, so
// its failure can be told apart from the active-window query's.
const windowPIDProperty = "_NET_WM_PID"

// supportingWMCheck is the property whose two-step self-reference proves a
// window manager is running now rather than having run once.
const supportingWMCheck = "_NET_SUPPORTING_WM_CHECK"

// rootHasLiveWindowManager completes the EWMH handshake through xprop —
// deliberately a different tool than the code under test, since reading the
// same properties through the same bridge would make the assertion circular.
//
// It follows the handshake rather than checking _NET_SUPPORTED for the reason
// the fix exists: root-window properties outlive the client that wrote them, so
// a window manager that died leaves its advertisements behind and only the
// window named here — destroyed with its connection — reports the truth.
func rootHasLiveWindowManager(t *testing.T) bool {
	t.Helper()

	xprop, err := exec.LookPath("xprop")
	if err != nil {
		t.Skip("xprop is not installed; cannot establish the session independently")
	}

	child := windowIDIn(xpropOutput(t, xprop, "-root", supportingWMCheck))
	if child == "" {
		return false
	}

	return windowIDIn(xpropOutput(t, xprop, "-id", child, supportingWMCheck)) == child
}

// xpropOutput runs xprop with args and returns its output, empty when it fails.
func xpropOutput(t *testing.T, xprop string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), xpropTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, xprop, args...).CombinedOutput()
	if err != nil {
		return ""
	}

	return string(out)
}

// windowIDIn pulls the "window id # 0x..." xprop prints for a WINDOW property,
// or empty when the line is anything else — including "not found".
func windowIDIn(out string) string {
	_, rest, found := strings.Cut(out, "window id # ")
	if !found {
		return ""
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}
