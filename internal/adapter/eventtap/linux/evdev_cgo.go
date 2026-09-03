//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

const (
	waylandEvdevEventBufferSize           = 128
	waylandEvdevModifierReleasePollPeriod = 5 * time.Millisecond
	waylandEvdevPreGrabHoldPollPeriod     = 50 * time.Millisecond
	waylandEvdevHotplugBufSize            = 4096
	waylandEvdevHotplugSettleDelay        = 100 * time.Millisecond
	waylandEvdevHotplugPollInterval       = 500 * time.Millisecond
)

// The two bounds on how long a mode activation waits for the keyboard to go
// quiet before grabbing it. Waiting at all is #1087's first line of defense:
// grabbing while a key is held routes that key's release to our fd alone, so
// libinput never sees it, considers the key down forever and eats its next
// press. The other two lines are the ones that make *these* bounds affordable —
// initialKeys suppresses the kernel's replay of a key already held at grab
// time, and shutdownEvdevSession injects a synthetic release for it on the way
// out — so a wait that expires falls back onto a handled path rather than off a
// cliff. That is what lets both of these be bounds instead of conditions.
//
// They differ because what is being waited for differs, and because the cost of
// giving up differs with it.
const (
	// waylandEvdevModifierReleaseTimeout bounds the wait for held *modifiers*.
	// It is the long one: a modifier whose release we swallow is stuck across
	// every application the user touches next, not just this mode, so it is
	// worth waiting out an activation chord that is being held. What it may not
	// be is unbounded, which is what it was — the loop had no deadline at all,
	// so a modifier the kernel reports as held (a chord the user rests on, a
	// key wedged by something else) left the mode active, the overlay drawn and
	// the keyboard never grabbed, with everything typed going to the focused
	// application and nothing said about it. `platform/linux/AGENTS.md` has the
	// rule this broke: never block on the eventtap goroutine.
	waylandEvdevModifierReleaseTimeout = 5 * time.Second

	// waylandEvdevPreGrabHoldTimeout bounds the wait for ordinary keys, and is
	// short because by the time it runs the modifier wait above has already
	// finished — so what is still down is a plain key, whose worst case is one
	// key eaten rather than a modifier stuck under everything.
	//
	// Five seconds here was the reported bug (a user mashing keys as the mode
	// came up, seeing "the grid clearly not moving… when I stop for a second,
	// it seems to fix the issue"): someone typing through an activation holds a
	// key at almost every poll, so the wait runs to its deadline and the mode
	// takes no input until it does. Waiting is futile in exactly that case —
	// the next poll finds another key down, and the grab that eventually
	// happens is a grab with a key held either way. Half a second is long
	// enough for the release of a deliberate keypress, which is what waiting is
	// for, and short enough that typing into an activation costs a beat rather
	// than the five seconds it cost before.
	waylandEvdevPreGrabHoldTimeout = 500 * time.Millisecond
)

// waylandEvdevKeyboardActive reports whether an evdev keyboard grab is currently
// held, i.e. keys are captured directly from the input devices rather than via
// the overlay layer-surface's keyboard grab. When true, the overlay must NOT
// request exclusive keyboard focus: on wlroots compositors (niri, Sway, …) a
// layer-surface keyboard grab deactivates the focused app's toplevel, which
// makes a hints refresh re-read the wrong "focused window" and tear the overlay
// down. See IsWaylandEvdevKeyboardActive and its use in the indicator polling.
var waylandEvdevKeyboardActive atomic.Bool

// IsWaylandEvdevKeyboardActive reports whether keys are currently being captured
// via an evdev grab (so the overlay's keyboard grab is redundant and must stay
// off). False on non-Wayland sessions, when the grab failed, and between modes.
func IsWaylandEvdevKeyboardActive() bool {
	return waylandEvdevKeyboardActive.Load()
}

// waylandEvdevGrabGeneration counts the grabs that have ended. It is what lets a
// passive reader of the same devices know that events happened which it did not
// see: while a grab is held it receives nothing at all, so it cannot observe the
// grab directly and cannot tell a quiet keyboard from a captured one.
//
// Comparing the generation it last saw against this one on the next event it does
// receive answers "has a mode been and gone since I last looked", which is
// exactly when everything it believes about which keys are down may be wrong —
// in both directions, since it missed presses and releases alike.
var waylandEvdevGrabGeneration atomic.Uint64

// endEvdevGrabGeneration records that a grab has ended. Paired with the flag
// above, and set in the same place, so no exit path can clear one and not the
// other.
func endEvdevGrabGeneration() {
	waylandEvdevKeyboardActive.Store(false)
	waylandEvdevGrabGeneration.Add(1)
}

var (
	errWaylandEvdevUnavailable = errors.New("wayland evdev capture unavailable")
	errWaylandEvdevGrabFailed  = errors.New("wayland evdev grab failed")
)

const (
	waylandEvdevDeviceNameSize = 256
	evdevMaxPressedKeys        = 256
)

const waylandEvdevBusVirtual = 0x06

var knownVirtualDevices = []string{"kanata"}

// neruInjectionDevicePrefix identifies Neru's own synthetic uinput devices
// (e.g. "neru-keyboard" from key feeding, "neru-scroll"). Capture must never
// grab these: grabbing our injection keyboard would swallow fed keys before
// they reach the focused app.
const neruInjectionDevicePrefix = "neru-"

func isNeruInjectionDevice(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), neruInjectionDevicePrefix)
}

func isUinputVirtualDevice(fd C.int, name string) bool {
	bustype := int(C.neru_evdev_get_bustype(fd))
	if bustype == waylandEvdevBusVirtual {
		return true
	}

	if name == "" {
		return false
	}

	lower := strings.ToLower(name)
	for _, known := range knownVirtualDevices {
		if strings.Contains(lower, known) {
			return true
		}
	}

	return false
}

type waylandEvdevEvent struct {
	eventType uint16
	code      uint16
	value     int32
}

type waylandEvdevKeyState struct {
	modifiers evdevModifierState
	pressed   map[uint16]bool
	// initialKeys tracks keys that were already physically held when
	// the event tap was (re-)enabled.  The kernel replays press events
	// via SYN_DROPPED after EVIOCGRAB; we suppress dispatch for these
	// until the physical release (removing from initialKeys).
	initialKeys map[uint16]bool
	// releasedDuringGrab is a subset of initialKeys: keys that were
	// released while the evdev grab was active and whose release events
	// never reached libinput.  We inject synthetic releases for these
	// at shutdown so libinput's per-keycode hw_is_key_down is cleared.
	releasedDuringGrab map[uint16]bool
	// grabGeneration is the value of waylandEvdevGrabGeneration this state was
	// last reconciled against, and belongs to the passive listener alone — see
	// needsReconcileAfterGrab for why the tap has no use for it.
	grabGeneration uint64
	// passthroughHeld maps a base keycode that is currently being passed
	// through to the focused app to the modifier names we pressed on the
	// virtual keyboard for it. The modifiers stay held (refcounted by the
	// wlroots modifier dispatcher) across the whole press→repeat→release
	// span so auto-repeat works without flapping the modifier, and are
	// released on the physical key-up. Presence also marks the release as
	// "already passed through" so no stray key-up leaks into Neru.
	passthroughHeld map[uint16][]string
}

// modifierTransition is what tracking a modifier key event turned out to mean.
//
// It exists because both readers of the devices track modifiers the same way and
// answer the third case differently, so the shared part is the tracking and the
// difference is what the caller does with this.
type modifierTransition int

const (
	// modifierHeld is a press: the modifier is now down. A press for a key
	// already tracked as down is the kernel repeating itself and still answers
	// this, having counted nothing twice.
	modifierHeld modifierTransition = iota
	// modifierDropped is the release of a press this state saw.
	modifierDropped
	// modifierReleaseUnmatched is a release with no press behind it, so nothing
	// was counted and nothing was decremented. It means this reader missed
	// events — the other one had the devices grabbed — and the caller decides
	// whether that is expected (the tap, whose grab has just started) or a
	// picture worth rebuilding (the listener, whose count would otherwise go
	// negative and stay there).
	modifierReleaseUnmatched
)

// needsReconcileAfterGrab reports whether a grab has begun and ended since this
// state was last reconciled, and records the generation either way.
//
// It is how the passive listener learns that its picture of the keyboard is stale:
// a grab hands it no events at all, so the arithmetic it does on the events it
// *does* get silently describes a keyboard it stopped watching. Both directions go
// wrong across such a window — a press it missed leaves a modifier uncounted, a
// release it missed leaves one counted forever — and only the second announces
// itself (as an unmatched release). This catches both, at the first event after
// the mode is gone.
//
// The tap never needs it: it builds a fresh state for each grab it holds, so it
// cannot be behind one.
func (state *waylandEvdevKeyState) needsReconcileAfterGrab(generation uint64) bool {
	if state.grabGeneration == generation {
		return false
	}

	state.grabGeneration = generation

	return true
}

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

type waylandEvdevCapture struct {
	files  []*os.File
	events chan waylandEvdevEvent
	logger *zap.Logger

	closeOnce        sync.Once
	done             sync.WaitGroup
	grabbed          bool
	startReadersOnce sync.Once

	deviceMu      sync.Mutex
	inotifyFd     int
	hotplugStopCh chan struct{}
	hotplugOnce   sync.Once
	hotplugWg     sync.WaitGroup

	// xkbState holds a C xkb_state initialized from the compositor's keymap.
	// Used to resolve evdev scan codes to key names that respect XKB options.
	xkbState unsafe.Pointer // *C.neru_xkb_state
}
