//go:build linux

package linux

import "github.com/y3owk1n/neru/internal/ports"

// x11HeldHotkeys is the loop's record of which grabbed keys are down, by
// keycode, so a hold is reported as one press and one release.
//
// Keyed by keycode alone, and not by the chord the grab named, because the
// release has to be found when the modifier has already come up: the state on
// that KeyRelease no longer spells the chord, and a lookup by chord would leave
// the binder repeating a key nobody holds.
type x11HeldHotkeys map[uint32]ports.HotkeyID

// press records that a grabbed key went down and reports whether it is a new
// hold. With detectable autorepeat the server reports a held key as further
// KeyPress events with no release between; those answer false and fire nothing,
// the way the macOS tap's `down` flag folds them.
func (h x11HeldHotkeys) press(keycode uint32, id ports.HotkeyID) bool {
	if _, down := h[keycode]; down {
		return false
	}

	h[keycode] = id

	return true
}

// release ends the hold on a keycode and returns the hotkey it belonged to.
// A release for a key this never saw go down (held before the grab existed)
// answers false.
func (h x11HeldHotkeys) release(keycode uint32) (ports.HotkeyID, bool) {
	id, down := h[keycode]
	if down {
		delete(h, keycode)
	}

	return id, down
}
