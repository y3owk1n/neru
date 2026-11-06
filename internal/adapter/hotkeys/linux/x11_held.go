//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/ports"

// x11HeldHotkeys is the loop's record of which grabbed keys are down, by
// keycode, so a hold is reported as one press and one release. Keyed by
// keycode, not chord, because the modifier may already be up at KeyRelease,
// and a chord lookup would leave the binder repeating a key nobody holds.
type x11HeldHotkeys map[uint32]ports.HotkeyID

// press records that a grabbed key went down and reports whether it is a new
// hold. With detectable autorepeat a held key repeats KeyPress with no
// release between, and those answer false, like the macOS tap's `down` flag.
func (h x11HeldHotkeys) press(keycode uint32, id ports.HotkeyID) bool {
	if _, down := h[keycode]; down {
		return false
	}

	h[keycode] = id

	return true
}

// release ends the hold on a keycode and returns its hotkey. A key held
// before the grab existed answers false.
func (h x11HeldHotkeys) release(keycode uint32) (ports.HotkeyID, bool) {
	id, down := h[keycode]
	if down {
		delete(h, keycode)
	}

	return id, down
}
