//go:build integration && linux && cgo

package linux

import (
	"os"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// x11HotkeyHold is how long the test keeps the activating hotkey down after the
// tap starts. It is longer than the second after which the tap logs that it is
// still waiting, so the tap has to keep waiting past that point.
const x11HotkeyHold = 1500 * time.Millisecond

// x11GrabSettle is how long the tap gets to take the keyboard once the hotkey
// comes up.
const x11GrabSettle = 200 * time.Millisecond

// x11KeysWait bounds how long the test waits for the tap to dispatch a key.
const x11KeysWait = 2 * time.Second

// TestEventTap_X11_GrabsTheKeyboardOnceTheActivatingHotkeyIsReleased pins that
// a mode started by a hotkey receives its keys on X11. The hotkey is still held
// when the tap asks for the keyboard, and the server refuses the grab while
// another client's passive grab on that key is active. A tap that gave up on
// the first refusal, or after a fixed wait the user outlasted, left every key
// the user then typed with the focused window.
func TestEventTap_X11_GrabsTheKeyboardOnceTheActivatingHotkeyIsReleased(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("DISPLAY") == "" || platform.DetectLinuxBackend().IsWayland() {
		t.Skip("skipping: no X11 session; this drives the X11 keyboard grab")
	}

	release, blocked, err := holdX11GrabbedKey("AC05")
	if err != nil {
		t.Fatalf("holding the hotkey under another client's grab: %v", err)
	}

	if !blocked {
		release()
		t.Fatal(
			"holding a grabbed key did not block a keyboard grab, so this test would prove nothing",
		)
	}

	keys := make(chan string, 64)
	eventTap := NewEventTap(func(key string) { keys <- key }, nil)

	eventTap.Enable()
	defer eventTap.Destroy()

	time.Sleep(x11HotkeyHold)
	release()
	time.Sleep(x11GrabSettle)

	err = pressX11Keys("AD01")
	if err != nil {
		t.Fatalf("typing into the mode: %v", err)
	}

	select {
	case <-keys:
	case <-time.After(x11KeysWait):
		t.Fatal(
			"the tap dispatched no key after the hotkey came up, so the mode's keys reached the focused window",
		)
	}
}
