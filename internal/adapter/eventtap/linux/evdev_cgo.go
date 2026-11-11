// Shared vocabulary of the Wayland evdev path: the event shape the readers hand
// on, the held-key picture both consumers keep, and the device predicates.

//go:build linux && cgo

package linux

import (
	"errors"
	"strings"
	"time"
)

const (
	waylandEvdevEventBufferSize     = 256
	waylandEvdevHotplugBufSize      = 4096
	waylandEvdevHotplugSettleDelay  = 100 * time.Millisecond
	waylandEvdevHotplugPollInterval = 500 * time.Millisecond

	// waylandEvdevKeymapPollInterval is how often the proxy asks whether the
	// compositor replaced the keymap. Layout changes are rare and a stale name
	// costs one mistyped binding. Every mode start also checks, for free.
	waylandEvdevKeymapPollInterval = 2 * time.Second

	// waylandEvdevIdlePollInterval is how often a keyboard found with a key
	// down is asked whether the key has come up, so it can be grabbed. Paid
	// once per device (waitIdle), never per activation.
	waylandEvdevIdlePollInterval = 10 * time.Millisecond

	// waylandEvdevProxyProbeInterval is how often the proxy checks whether
	// another process grabbed one of its own devices (probeOwnDevices). It
	// bounds how long the keyboard is dead when a remapper closes the loop.
	waylandEvdevProxyProbeInterval = 500 * time.Millisecond

	// waylandEvdevYieldGrace is how long a physical keyboard yielded to a
	// starting remapper stays with the compositor before an unclaimed one is
	// taken back. Kanata grabs its inputs two seconds after creating its
	// output device, so this adds room for its idle wait.
	waylandEvdevYieldGrace = 3 * time.Second
)

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

// neruInjectionDevicePrefix identifies Neru's own uinput devices (the proxy
// keyboard and pointer, "neru-keyboard", "neru-scroll"). Capture must never
// grab these. Grabbing the proxy loops every key back into the reader, and
// grabbing the feed keyboard swallows fed keys.
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
	// modifierHeld is a press. A press for a key already down is a kernel
	// repeat and still answers this, counting nothing twice.
	modifierHeld modifierTransition = iota
	// modifierDropped is the release of a press this state saw.
	modifierDropped
	// modifierReleaseUnmatched is a release with no press behind it, so
	// nothing changes. The proxy sees one for a key held before the daemon
	// started, and a mode session for every key held before the mode, the
	// usual case under an activation chord.
	modifierReleaseUnmatched
)

// trackModifier records a modifier key event against the held-key picture and
// reports what it meant.
//
// The refcount is per modifier name, so two Shift keys held together count
// twice and releasing one leaves the modifier down. A press for a code
// already down is a kernel repeat and is not counted again.
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
