//go:build integration && linux && cgo

package linux

import (
	"os"
	"testing"
	"time"
)

// TestWaylandEvdevCapture_PollKeymap_ReadsAKeyboardTheSeatGainsLater pins that
// key names follow the compositor's keymap when the seat had no keyboard as the
// capture started, as in a headless session. Asking such a seat for a keyboard
// is a protocol error that drops the connection, which left names on the
// built-in us table for the daemon's lifetime. On a seat that already has a
// keyboard the keymap is there from the start and the test passes at once.
func TestWaylandEvdevCapture_PollKeymap_ReadsAKeyboardTheSeatGainsLater(t *testing.T) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("needs a Wayland session")
	}

	capture := &waylandEvdevCapture{}
	capture.refreshXkbState()

	if capture.xkbState == nil {
		t.Fatal("the keymap reader did not come up on this seat")
	}

	t.Cleanup(capture.destroyTestXkb)

	keyboard := addTestSeatKeyboard()
	if keyboard == nil {
		t.Skip("the compositor offers no virtual keyboard")
	}

	t.Cleanup(keyboard.remove)

	// liveKeyName has no fallback, so a name means lookups resolve through
	// the compositor's keymap.
	deadline := time.Now().Add(5 * time.Second)
	for capture.liveKeyName(evdevKeyQ) == "" {
		if time.Now().After(deadline) {
			t.Fatal("keys never resolved through a keymap after the seat gained a keyboard")
		}

		capture.pollKeymap()
		time.Sleep(50 * time.Millisecond)
	}
}
