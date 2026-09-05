//go:build linux

package linux

// evdevKeyCodeCount is KEY_CNT: one past the highest evdev key code.
const evdevKeyCodeCount = 0x300

// evdevBitsPerWord is the width of one word of a key bitmap, here and in the
// kernel's EVIOCGKEY answer.
const evdevBitsPerWord = 64

// forwardRule decides, per key event, whether the proxy re-emits it to the
// compositor or keeps it. It holds one bit per key code: set while a press this
// rule forwarded is still down.
//
// The whole of it is one invariant, and the invariant is what makes an instant
// grab safe: **a release is forwarded exactly when its press was.** A press is
// forwarded whenever no mode is capturing, and withheld otherwise; its release
// then follows the press wherever it went, and a repeat follows the press too.
// So a mode that starts under a held activation chord costs nothing — the
// chord's presses went to the compositor, so their releases do as well, and the
// compositor's picture of the keyboard is never wrong in either direction. That
// is the property the previous design bought by waiting for the chord to be
// released before grabbing, with the mode's first hundred milliseconds of input
// going to the focused application instead (#1087 and what it cost).
//
// It is pure Go with no lock: the proxy's run goroutine is the only caller.
type forwardRule struct {
	down [evdevKeyCodeCount / evdevBitsPerWord]uint64
}

// seed marks code as down and forwarded without an event: the kernel reported
// it held when its device was grabbed, so the compositor saw the press and is
// owed the release.
func (r *forwardRule) seed(code uint16) {
	r.set(code)
}

// press decides a key-down. withhold is whether a consumer wants the press for
// itself; a press for a key already forwarded and down (a second keyboard
// pressing the same key) is forwarded regardless, because its release will be.
func (r *forwardRule) press(code uint16, withhold bool) bool {
	if withhold && !r.isDown(code) {
		return false
	}

	r.set(code)

	return true
}

// repeat decides a kernel auto-repeat: it goes wherever the press went.
func (r *forwardRule) repeat(code uint16) bool {
	return r.isDown(code)
}

// release decides a key-up: forwarded exactly when the press was.
func (r *forwardRule) release(code uint16) bool {
	forwarded := r.isDown(code)
	r.clear(code)

	return forwarded
}

func (r *forwardRule) isDown(code uint16) bool {
	if int(code) >= evdevKeyCodeCount {
		return false
	}

	return r.down[code/evdevBitsPerWord]&(1<<(code%evdevBitsPerWord)) != 0
}

func (r *forwardRule) set(code uint16) {
	if int(code) >= evdevKeyCodeCount {
		return
	}

	r.down[code/evdevBitsPerWord] |= 1 << (code % evdevBitsPerWord)
}

func (r *forwardRule) clear(code uint16) {
	if int(code) >= evdevKeyCodeCount {
		return
	}

	r.down[code/evdevBitsPerWord] &^= 1 << (code % evdevBitsPerWord)
}
