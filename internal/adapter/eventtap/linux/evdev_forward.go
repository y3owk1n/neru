//go:build linux

package linux

// evdevKeyCodeCount is KEY_CNT: one past the highest evdev key code.
const evdevKeyCodeCount = 0x300

// evdevBitsPerWord is the width of one word of a key bitmap, as in the
// kernel's EVIOCGKEY answer.
const evdevBitsPerWord = 64

// forwardRule decides, per key event, whether the proxy re-emits it to the
// compositor or keeps it. It counts, per key code, the forwarded presses
// still down across every keyboard.
//
// One invariant makes an instant grab safe. A release is forwarded exactly
// when its press was. A press is forwarded when no mode is capturing
// and withheld otherwise, except a modifier's press, which always is
// (handleKey). Repeats and the release follow the press. So a mode starting
// under a held activation chord costs nothing, and the compositor's picture
// of the keyboard is never wrong. The earlier design waited for the chord to
// come up before grabbing, and the mode's first input went to the focused
// app (#1087).
//
// A count, not a bit, because the proxy stands in for several keyboards. The
// same key held on two is one key down to the compositor, released when the
// last lets go, so only that release is forwarded.
//
// Pure Go with no lock, because the run goroutine is the only caller.
type forwardRule struct {
	down [evdevKeyCodeCount]uint8
}

// seed counts code as down and forwarded without an event, because the proxy
// re-emitted the press itself (forwardWithheld) and owes the release.
func (r *forwardRule) seed(code uint16) {
	r.add(code)
}

// press decides a key-down. withhold is whether a consumer wants the press. A
// key already forwarded and down (pressed on a second keyboard) is forwarded
// regardless, because its release will be.
func (r *forwardRule) press(code uint16, withhold bool) bool {
	if withhold && !r.isDown(code) {
		return false
	}

	r.add(code)

	return true
}

// repeat decides a kernel auto-repeat, which goes wherever the press went.
func (r *forwardRule) repeat(code uint16) bool {
	return r.isDown(code)
}

// release decides a key-up. It is forwarded exactly when the press was, and
// for a key held on several keyboards only for the last to let go.
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
