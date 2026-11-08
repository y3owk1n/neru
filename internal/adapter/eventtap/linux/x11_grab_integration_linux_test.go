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

	err = pressX11Keys(x11KeyQ)
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

// x11NoKeyWait is how long the test waits to be sure the tap dispatched
// nothing, and x11YieldedKeys how many keys it types with the tap aside.
const (
	x11NoKeyWait   = 300 * time.Millisecond
	x11YieldedKeys = 40
)

// TestEventTap_X11_YieldKeyboard_SendsTypedKeysToTheFocusedWindow pins that a
// key Neru types while a mode holds the keyboard reaches the app. X11 cannot
// tag a key as Neru's, so under the grab it came straight back to the tap,
// and vim mode's edits and typing never reached the window and looped.
func TestEventTap_X11_YieldKeyboard_SendsTypedKeysToTheFocusedWindow(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("DISPLAY") == "" || platform.DetectLinuxBackend().IsWayland() {
		t.Skip("skipping: no X11 session; this drives the X11 keyboard grab")
	}

	window, err := openX11FocusedWindow()
	if err != nil {
		t.Fatalf("opening a focused window: %v", err)
	}
	defer window.close()

	keys := make(chan string, 64)
	eventTap := NewEventTap(func(key string) { keys <- key }, nil)

	eventTap.Enable()
	defer eventTap.Destroy()

	time.Sleep(x11GrabSettle)

	err = pressX11Keys(x11KeyQ)
	if err != nil {
		t.Fatalf("typing into the mode: %v", err)
	}

	select {
	case <-keys:
	case <-time.After(x11KeysWait):
		t.Fatal("the tap dispatched no key, so it never held the keyboard")
	}

	// The release follows as a key-up event.
	time.Sleep(x11NoKeyWait)

	for len(keys) > 0 {
		<-keys
	}

	window.keyPresses()

	// Back to back, as vim mode types a word, so a key can arrive while the
	// tap takes the keyboard back from the one before.
	for range x11YieldedKeys {
		err = eventTap.YieldKeyboard(func() error { return pressX11Keys(x11KeyQ) })
		if err != nil {
			t.Fatalf("typing a key with the tap stepped aside: %v", err)
		}
	}

	if got := window.keyPresses(); got != x11YieldedKeys {
		t.Fatalf("the focused window received %d key presses, want the %d Neru typed",
			got, x11YieldedKeys)
	}

	select {
	case key := <-keys:
		t.Fatalf("the tap dispatched %q, the key Neru typed into the window", key)
	case <-time.After(x11NoKeyWait):
	}

	err = pressX11Keys(x11KeyQ)
	if err != nil {
		t.Fatalf("typing into the mode after the yield: %v", err)
	}

	select {
	case <-keys:
	case <-time.After(x11KeysWait):
		t.Fatal("the tap dispatched no key after the yield, so it never took the keyboard back")
	}

	if got := window.keyPresses(); got != 0 {
		t.Fatalf("the focused window received %d key presses after the yield, want none", got)
	}
}
