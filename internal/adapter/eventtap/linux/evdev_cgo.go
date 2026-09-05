// Shared vocabulary of the Wayland evdev path: the event shape the readers hand
// on, the held-key picture both consumers keep, and the device predicates.

//go:build linux && cgo

package linux

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

const (
	waylandEvdevEventBufferSize     = 256
	waylandEvdevHotplugBufSize      = 4096
	waylandEvdevHotplugSettleDelay  = 100 * time.Millisecond
	waylandEvdevHotplugPollInterval = 500 * time.Millisecond

	// waylandEvdevKeymapPollInterval is how often the proxy asks its Wayland
	// connection whether the compositor replaced the keymap. Layout changes are
	// rare and a stale name costs one mistyped binding, so this is slow; the
	// same check runs at every mode start, where it is free.
	waylandEvdevKeymapPollInterval = 2 * time.Second

	// waylandEvdevIdlePollInterval is how often a keyboard found with a key
	// down is asked whether the key has come up, so it can be grabbed. Paid
	// once per device (waitIdle), never per activation.
	waylandEvdevIdlePollInterval = 10 * time.Millisecond

	// waylandEvdevProxyProbeInterval is how often the proxy asks whether
	// another process has grabbed one of its own devices (probeOwnDevices).
	// Two ioctls per tick, nothing on the keystroke path; it bounds how long
	// the keyboard is dead when a remapper closes the loop.
	waylandEvdevProxyProbeInterval = 500 * time.Millisecond

	// waylandEvdevYieldGrace is how long a physical keyboard yielded to a
	// remapper that just started stays with the compositor before the capture
	// takes back one the remapper did not want. Kanata grabs its inputs two
	// seconds after creating its output device (its startup delay), so this
	// is that with room for its idle wait.
	waylandEvdevYieldGrace = 3 * time.Second
)

// waylandEvdevKeyboardActive reports whether a mode session currently owns the
// keys, i.e. presses are captured for the mode rather than re-emitted to the
// compositor. When true, the overlay must NOT request exclusive keyboard focus:
// on wlroots compositors (niri, Sway, …) a layer-surface keyboard grab
// deactivates the focused app's toplevel, which makes a hints refresh re-read
// the wrong "focused window" and tear the overlay down.
var waylandEvdevKeyboardActive atomic.Bool

// IsWaylandEvdevKeyboardActive reports whether keys are currently being captured
// by the evdev proxy for a mode (so the overlay's keyboard grab is redundant and
// must stay off). False on non-Wayland sessions, when the proxy is unavailable,
// and between modes.
func IsWaylandEvdevKeyboardActive() bool {
	return waylandEvdevKeyboardActive.Load()
}

var (
	errWaylandEvdevUnavailable  = errors.New("wayland evdev capture unavailable")
	errWaylandEvdevProxyStopped = errors.New("wayland evdev proxy stopped")
	errWaylandEvdevShortWrite   = errors.New("short write to the proxy device")
	errWaylandEvdevProxyGrabbed = errors.New("another process holds a proxy device")
	errWaylandEvdevPassive      = errors.New(
		"wayland evdev proxy is passive: /dev/uinput is not writable, so keys cannot be captured",
	)
	errWaylandEvdevYielded = errors.New(
		"wayland evdev proxy has yielded the keyboards to a remapper that just started, " +
			"and takes back in a moment any it does not claim",
	)
	errWaylandEvdevGrabPending = errors.New(
		"wayland evdev proxy holds no keyboard yet: every keyboard has a key down, " +
			"and each is held once its key is released",
	)
)

const waylandEvdevDeviceNameSize = 256

// neruInjectionDevicePrefix identifies Neru's own synthetic uinput devices
// (the proxy keyboard and pointer, "neru-keyboard" from key feeding,
// "neru-scroll").
// Capture must never grab these: grabbing the proxy would loop every key back
// into the reader, and grabbing the feed keyboard would swallow fed keys before
// they reach the focused app.
const neruInjectionDevicePrefix = "neru-"

func isNeruInjectionDevice(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), neruInjectionDevicePrefix)
}

const (
	evdevEventSyn uint16 = 0x00
	evdevEventRel uint16 = 0x02
	evdevEventMsc uint16 = 0x04
	evdevEventLed uint16 = 0x11

	evdevSynReport uint16 = 0
)

type waylandEvdevEvent struct {
	eventType uint16
	code      uint16
	value     int32
}

// waylandEvdevKeyState is a picture of which keys and modifiers are down, kept
// by feeding it every key event.
type waylandEvdevKeyState struct {
	modifiers evdevModifierState
	pressed   map[uint16]bool
}

func newWaylandEvdevKeyState() waylandEvdevKeyState {
	return waylandEvdevKeyState{pressed: make(map[uint16]bool)}
}

// modifierTransition is what tracking a modifier key event turned out to mean.
type modifierTransition int

const (
	// modifierHeld is a press: the modifier is now down. A press for a key
	// already tracked as down is the kernel repeating itself and still answers
	// this, having counted nothing twice.
	modifierHeld modifierTransition = iota
	// modifierDropped is the release of a press this state saw.
	modifierDropped
	// modifierReleaseUnmatched is a release with no press behind it, so nothing
	// was counted and nothing was decremented. The proxy's own picture sees one
	// only for a key held before the daemon started; a mode session sees one
	// for every key held before the mode did, which is the ordinary way a mode
	// starts under its activation chord.
	modifierReleaseUnmatched
)

// trackModifier records a modifier key event against the held-key picture and
// reports what it meant.
//
// The refcount is per modifier *name*, not per key, so the two Shift keys held
// together count twice and releasing one leaves the modifier down. A press for a
// code already down is not counted again: the kernel repeats a held key, and
// counting each repeat would leave the modifier held after the single release
// that follows.
func (state *waylandEvdevKeyState) trackModifier(
	code uint16,
	modifier string,
	isDown bool,
) modifierTransition {
	switch {
	case isDown:
		alreadyTracked := state.pressed[code]
		state.trackKey(code, true)

		if !alreadyTracked {
			state.modifiers.update(modifier, true)
		}

		return modifierHeld
	case state.pressed[code]:
		state.trackKey(code, false)
		state.modifiers.update(modifier, false)

		return modifierDropped
	default:
		return modifierReleaseUnmatched
	}
}

func (state *waylandEvdevKeyState) trackKey(code uint16, isDown bool) {
	if state == nil {
		return
	}

	if state.pressed == nil {
		state.pressed = make(map[uint16]bool)
	}

	if isDown {
		state.pressed[code] = true

		return
	}

	delete(state.pressed, code)
}
