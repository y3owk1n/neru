//go:build linux

package linux

import "testing"

// A held hotkey reaches the binder as one press and one release. The server
// reports the hold as repeated KeyPress events (detectable autorepeat), and
// the release may arrive after the chord's modifier has come up.
func TestX11HeldHotkeys_AHoldIsOnePressAndOneRelease(t *testing.T) {
	t.Parallel()

	const keycode = 47

	held := x11HeldHotkeys{}

	if !held.press(keycode, 1) {
		t.Fatal("the first press was not reported as a new hold")
	}

	for range 3 {
		if held.press(keycode, 1) {
			t.Fatal(
				"an autorepeat press was reported as a new hold; the binder would restart its repeat",
			)
		}
	}

	id, down := held.release(keycode)
	if !down || id != 1 {
		t.Fatalf("release = (%d, %v), want (1, true)", id, down)
	}

	if _, down := held.release(keycode); down {
		t.Fatal("a second release found the key still down")
	}

	if !held.press(keycode, 1) {
		t.Fatal("a press after the release was not a new hold")
	}
}
