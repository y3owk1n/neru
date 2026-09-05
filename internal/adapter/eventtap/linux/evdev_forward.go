//go:build linux

package linux

// evdevKeyCodeCount is KEY_CNT: one past the highest evdev key code.
const evdevKeyCodeCount = 0x300

// evdevBitsPerWord is the width of one word of a key bitmap, as in the
// kernel's EVIOCGKEY answer.
const evdevBitsPerWord = 64

// forwardRule decides, per key event, whether the proxy re-emits it to the
// compositor or keeps it. It holds one count per key code: how many forwarded
// presses of that key are still down, across every keyboard.
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
// A count rather than a bit, because the proxy is one keyboard standing in for
// several: the same key held on two of them is one key down to the compositor,
// which sees it go up when the last of them lets go, as it would with a single
// keyboard. Their presses are forwarded either way (the kernel drops the second
// on a device that already has the key down); only the last release is.
//
// It is pure Go with no lock: the proxy's run goroutine is the only caller.
type forwardRule struct {
	down [evdevKeyCodeCount]uint8
}

// seed counts code as down and forwarded without an event: the proxy re-emitted
// the press itself (forwardWithheld), so the compositor saw it and is owed the
// release.
func (r *forwardRule) seed(code uint16) {
	r.add(code)
}

// press decides a key-down. withhold is whether a consumer wants the press for
// itself; a press for a key already forwarded and down (a second keyboard
// pressing the same key) is forwarded regardless, because its release will be.
func (r *forwardRule) press(code uint16, withhold bool) bool {
	if withhold && !r.isDown(code) {
		return false
	}

	r.add(code)

	return true
}

// repeat decides a kernel auto-repeat: it goes wherever the press went.
func (r *forwardRule) repeat(code uint16) bool {
	return r.isDown(code)
}

// release decides a key-up: forwarded exactly when the press was, and for a
// key held on more than one keyboard, only for the last one to let go.
func (r *forwardRule) release(code uint16) bool {
	if !r.isDown(code) {
		return false
	}

	r.down[code]--

	return r.down[code] == 0
}

func (r *forwardRule) isDown(code uint16) bool {
	return int(code) < evdevKeyCodeCount && r.down[code] > 0
}

func (r *forwardRule) add(code uint16) {
	if int(code) < evdevKeyCodeCount && r.down[code] < ^uint8(0) {
		r.down[code]++
	}
}
