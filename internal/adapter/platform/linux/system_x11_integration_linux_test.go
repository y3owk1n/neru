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
// CodeNotSupported / CodeActionFailed line this package draws — so the property
// that separates them is _NET_SUPPORTED, not _NET_ACTIVE_WINDOW. Openbox, which
// CI's X11 leg runs, is exactly the case that proves it: it advertises
// _NET_ACTIVE_WINDOW and then writes no such property until a window takes
// focus. Getting this backwards reports a healthy desktop as a broken one,
// which is the whole of #1495.
func TestX11FocusedApplicationPID_AWindowManagerIsNeverMistakenForNoWindowManager(
	t *testing.T,
) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X11 display; this asserts a rule about a live root window")
	}

	if !rootAdvertisesEWMH(t) {
		t.Skip("no EWMH window manager owns this display; nothing to mistake it for")
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
			"a display whose root advertises _NET_SUPPORTED answered %v (code %q); "+
				"under a live window manager the active-window query must not fail",
			err, derrors.GetCode(err),
		)
	}
}

// windowPIDProperty names the second property the focused-app path reads, so
// its failure can be told apart from the active-window query's.
const windowPIDProperty = "_NET_WM_PID"

// rootAdvertisesEWMH asks xprop whether a window manager has claimed EWMH,
// deliberately using a different tool than the code under test: reading
// _NET_SUPPORTED through the same bridge would make the assertion circular.
func rootAdvertisesEWMH(t *testing.T) bool {
	t.Helper()

	xprop, err := exec.LookPath("xprop")
	if err != nil {
		t.Skip("xprop is not installed; cannot establish the session independently")
	}

	ctx, cancel := context.WithTimeout(t.Context(), xpropTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, xprop, "-root", "_NET_SUPPORTED").CombinedOutput()
	if err != nil {
		return false
	}

	return strings.Contains(string(out), "_NET_ACTIVE_WINDOW")
}
