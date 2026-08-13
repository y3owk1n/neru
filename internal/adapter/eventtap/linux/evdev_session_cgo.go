// The Wayland evdev event-tap session: running a capture for a mode,
// dispatching key events through the shared vocabulary, chord
// passthrough, and orderly shutdown.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"errors"
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	linux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// initEvdevCapture initializes the persistent waylandEvdevCapture.
// A failed attempt can be retried later, allowing detection of newly
// connected keyboards after startup.
func (et *EventTap) initEvdevCapture() (*waylandEvdevCapture, error) {
	et.evdevWaylandCaptureInit.Lock()
	defer et.evdevWaylandCaptureInit.Unlock()

	if et.evdevWaylandCapture != nil {
		c, ok := et.evdevWaylandCapture.(*waylandEvdevCapture)
		if !ok {
			return nil, errWaylandEvdevUnavailable
		}

		return c, nil
	}

	wlCapture, capErr := newWaylandEvdevCapture(et.logger)
	if capErr != nil {
		if et.logger != nil {
			level := et.logger.Info
			if !errors.Is(capErr, errWaylandEvdevUnavailable) {
				level = et.logger.Warn
			}

			level(
				"Wayland evdev capture unavailable; falling back to overlay keyboard focus",
				zap.Error(capErr),
			)
		}

		return nil, capErr
	}

	et.evdevWaylandCapture = wlCapture

	return wlCapture, nil
}

// closeEvdevCapture closes the persistent evdev capture, releasing all file
// descriptors and stopping reader goroutines. It is safe to call multiple
// times — the underlying Close() uses sync.Once.
func (et *EventTap) closeEvdevCapture() {
	if et.evdevWaylandCapture == nil {
		return
	}

	capture, ok := et.evdevWaylandCapture.(*waylandEvdevCapture)
	if !ok {
		return
	}

	capture.Close()
	et.evdevWaylandCapture = nil
}

func (et *EventTap) runWaylandEvdev() bool {
	// Clear the evdev-active flag on every exit path (mode end / ungrab), so the
	// overlay may reclaim the keyboard grab if it ever becomes the fallback.
	defer waylandEvdevKeyboardActive.Store(false)

	// Get or create the persistent capture (initialized once, reused
	// across Enable/Disable cycles). This avoids re-scanning
	// /dev/input/event* devices on every mode activation, which was
	// the source of a mild delay before modes accepted input.
	capture, err := et.initEvdevCapture()
	if err != nil {
		return false
	}

	et.refreshEvdevXkbState(capture)

	// Only the Linux overlay backends can hold or release the keyboard.
	overlayCapture, _ := overlay.Get().(overlaymanager.KeyboardCaptureController)

	if stopped := et.waitForEvdevKeysReleased(capture, overlayCapture); stopped {
		return true
	}

	grabErr := capture.grabAll()
	if grabErr != nil {
		if et.logger != nil {
			et.logger.Warn(
				"Failed to grab Wayland evdev keyboards; falling back to overlay keyboard focus",
				zap.Error(grabErr),
			)
		}

		return false
	}

	// The evdev grab now owns the keyboard for this mode session, so the overlay
	// must stay keyboard-passive (see waylandEvdevKeyboardActive). Cleared by the
	// deferred Store(false) when this function returns on mode exit / ungrab.
	waylandEvdevKeyboardActive.Store(true)

	// Start reader goroutines on first invocation only; they run for
	// the entire lifetime of the capture (until EventTap.Destroy()).
	capture.startReadersOnce.Do(func() {
		capture.startReaders()
	})

	if overlayCapture != nil {
		// Keep the overlay keyboard-passive for the whole session and do NOT
		// restore it to EXCLUSIVE on exit: the evdev grab owns the keyboard, so a
		// layer-surface grab would only deactivate the focused app's toplevel on
		// wlroots. Every subsequent mode therefore also starts from NONE. The
		// non-evdev fallback (runWayland) raises EXCLUSIVE itself when it needs it.
		overlayCapture.SetKeyboardCaptureEnabled(false)
	}

	if et.logger != nil {
		et.logger.Info(
			"Using Wayland evdev keyboard capture",
			zap.Int("devices", len(capture.files)),
		)
	}

	// Drain any stale events that accumulated in the channel while
	// Neru was disabled between modes. These are events from other
	// applications that were pushed into the buffer when we were
	// ungrabbed. A labeled break is required here — plain break
	// inside select only exits the select, not the for loop.
	state := newEvdevSessionState(capture)

	for {
		select {
		case <-et.stopCh:
			et.shutdownEvdevSession(capture, &state)

			return true
		case event, ok := <-capture.events:
			if !ok {
				return true
			}

			et.handleWaylandEvdevEvent(&state, event)
		}
	}
}

func (et *EventTap) handleWaylandEvdevEvent(
	state *waylandEvdevKeyState,
	event waylandEvdevEvent,
) {
	if event.eventType != evdevEventKey {
		return
	}

	// Feed all key events to xkb_state so its internal state stays
	// consistent for key symbol resolution via XKB (respects options
	// like caps:swapescape set by the compositor).
	capture, _ := et.evdevWaylandCapture.(*waylandEvdevCapture)
	if capture != nil && capture.xkbState != nil {
		switch event.value {
		case evdevValuePress:
			C.neru_xkb_state_key((*C.neru_xkb_state)(capture.xkbState), C.uint16_t(event.code), 1)
		case evdevValueRelease:
			C.neru_xkb_state_key((*C.neru_xkb_state)(capture.xkbState), C.uint16_t(event.code), 0)
		}
	}

	// Resolve the modifier name through the XKB keymap so that compositor
	// options like ctrl:swapcaps and caps:swapescape are respected: when
	// XKB remaps a physical modifier to a different function, the handler
	// uses the remapped modifier name (or bypasses modifier handling when
	// the key is remapped to a non-modifier).
	modifier := et.xkbStateModifierName(capture, event.code)
	if modifier != "" {
		if event.value == evdevValueRepeat {
			return
		}

		isDown := event.value == evdevValuePress

		switch {
		case isDown:
			alreadyTracked := state.pressed[event.code]
			state.trackKey(event.code, true)
			if !alreadyTracked {
				state.modifiers.update(modifier, true)
			}
		case state.pressed[event.code]:
			state.trackKey(event.code, false)
			state.modifiers.update(modifier, false)
		default:
			// Release without a matching press (press happened before
			// fd was opened). Don't decrement — the count was never
			// incremented for this key, and doing so would drive it
			// negative, causing allZero() to return true prematurely.
			return
		}

		if et.consumeSyntheticModifierEvent(modifier, isDown) {
			return
		}

		if et.stickyToggleEnabled() && et.stickyDetectionArmed() {
			et.dispatchKey(keyvocab.ModifierToggleEvent(modifier, isDown))
		}

		// Re-arm detection when the modifier state reaches a clean slate,
		// matching macOS behavior where initial held-modifier releases from
		// an activation chord are not interpreted as sticky toggles.
		if !isDown && !et.stickyDetectionArmed() && state.modifiers.allZero() {
			et.stickyArmDetection()
		}

		return
	}

	switch event.value {
	case evdevValuePress:
		// If this key was already held when the event tap was e kernel's SYN_DROPPED state replay after
		// EVIOCGRAB. Track it in pressed (so subsequent repeats are not
		// silently consumed) but skip dispatch — the user did not press
		// it during this mode session. The initialKeys entry persists
		// until the physical release so repeats continue to be suppressed.
		state.trackKey(event.code, true)

		if state.initialKeys[event.code] {
			return
		}
	case evdevValueRelease:
		if state.initialKeys[event.code] {
			state.releasedDuringGrab[event.code] = true
			delete(state.initialKeys, event.code)
		}
		state.trackKey(event.code, false)

		// A key that was passed through never reached Neru as a press, so its
		// release must not leak a key-up into Neru either. Release the virtual
		// modifiers we were holding for it (refcounted, so a modifier shared
		// with another passthrough key or a sticky hold stays down).
		if mods, ok := state.passthroughHeld[event.code]; ok {
			delete(state.passthroughHeld, event.code)
			et.releasePassthroughModifiers(mods)

			return
		}

		key := et.xkbEvdevKeyName(capture, event.code)
		if key != "" {
			if keyUp := keyvocab.KeyUpEvent(key); keyUp != "" {
				et.dispatchKey(keyUp)
			}
		}

		return
	case evdevValueRepeat:
		if !state.pressed[event.code] {
			return
		}

		// Suppress repeat dispatch for keys that were held before mode
		// activation. The user must release and re-press to have the key
		// register as a fresh input in the active mode.
		if state.initialKeys[event.code] {
			return
		}
	default:
		return
	}

	// A key already owned by passthrough keeps routing its repeats to the app for
	// as long as it is physically held, regardless of the current modifier state.
	// Releasing a modifier mid-hold must not reclassify the key back into Neru
	// (the virtual modifier stays held until the physical key-up). Keyed by code,
	// so this runs before key-name resolution.
	if _, held := state.passthroughHeld[event.code]; held {
		et.passthroughEvdevChord(state, event.code, false)

		return
	}

	key := et.xkbEvdevKeyName(capture, event.code)
	if key == "" {
		return
	}

	key = keyvocab.NormalizeKey(state.modifiers.prefix() + key)
	if key == "" {
		return
	}

	// Modifier passthrough: when an unbound Ctrl/Alt/Cmd chord should reach the
	// focused application, re-inject it through the virtual keyboard (a distinct
	// device from the EVIOCGRAB'd physical keyboard, so it does not re-enter this
	// reader) and skip Neru's own dispatch. If injection fails, fall through to
	// normal dispatch rather than silently dropping the shortcut.
	if et.shouldPassthroughChord(key) && et.passthroughEvdevChord(state, event.code, true) {
		return
	}

	et.dispatchKey(key)
}

// passthroughEvdevChord re-injects a modifier chord to the focused application
// through the zwp_virtual_keyboard_v1 path and reports whether it was delivered.
// On the initial press it holds the currently-held modifiers down (refcounted by
// the wlroots modifier dispatcher) and taps the base keycode, records the hold so
// the physical release can drop those modifiers, and notifies the mode layer
// once. On auto-repeat it re-taps only the base key, leaving the modifiers held
// so the app sees a steadily-held modifier instead of it flapping around every
// repeat.
//
// It returns false when the essential injection (a held modifier or the base-key
// press) failed on the initial press, having rolled back any modifiers it pressed
// so none stays latched; the caller then falls back to normal dispatch rather
// than reporting a delivered shortcut. Returns true once the key is owned by
// passthrough — a dropped auto-repeat is tolerated, since the key stays owned and
// its modifiers held until the physical release either way.
func (et *EventTap) passthroughEvdevChord(
	state *waylandEvdevKeyState,
	code uint16,
	isPress bool,
) bool {
	if _, held := state.passthroughHeld[code]; held && !isPress {
		// Auto-repeat: modifiers are already held; just re-tap the base key.
		_ = linux.WaylandKeyEvent(uint32(code), true)
		_ = linux.WaylandKeyEvent(uint32(code), false)

		return true
	}

	mods := heldPassthroughModifiers(state)

	// Press the held modifiers, remembering which actually took so a
	// mid-sequence failure can be unwound without leaving one latched.
	pressed := make([]string, 0, len(mods))

	for _, mod := range mods {
		err := linux.WaylandModifierEvent(mod, true)
		if err != nil {
			et.releasePassthroughModifiers(pressed)

			return false
		}

		pressed = append(pressed, mod)
	}

	// The app acts on the key-down (the chord is delivered there); a failed
	// key-up only leaves cleanup pending, so only the down gates success.
	keyDownErr := linux.WaylandKeyEvent(uint32(code), true)
	if keyDownErr != nil {
		et.releasePassthroughModifiers(pressed)

		return false
	}

	keyUpErr := linux.WaylandKeyEvent(uint32(code), false)
	if keyUpErr != nil && et.logger != nil {
		// The chord was already delivered by the key-down; a rejected key-up is
		// not retried (the injection channel would have to recover), but log it
		// so a stuck virtual key is diagnosable.
		et.logger.Warn(
			"Failed to release passthrough key",
			zap.Uint16("keycode", code),
			zap.Error(keyUpErr),
		)
	}

	if state.passthroughHeld == nil {
		state.passthroughHeld = make(map[uint16][]string)
	}

	state.passthroughHeld[code] = mods

	et.firePassthroughCallback()

	return true
}

// releasePassthroughModifiers releases the given modifiers in reverse press
// order (refcounted, so a modifier shared with another passthrough key or a
// sticky hold stays down). Used both to unwind a failed press and to drop a
// held chord's modifiers on release/shutdown. A rejected release is not retried
// (a dead injection channel would keep failing), but is logged so a latched
// virtual modifier is diagnosable — matching the shutdown synthetic-release path.
func (et *EventTap) releasePassthroughModifiers(mods []string) {
	for _, mod := range slices.Backward(mods) {
		err := linux.WaylandModifierEvent(mod, false)
		if err != nil && et.logger != nil {
			et.logger.Warn(
				"Failed to release passthrough modifier",
				zap.String("modifier", mod),
				zap.Error(err),
			)
		}
	}
}

// passthroughModifierCount is the number of distinct modifier groups
// (shift/ctrl/alt/cmd) a re-injected chord can carry.
const passthroughModifierCount = 4

// heldPassthroughModifiers returns the canonical names of the modifiers
// currently held, in a stable shift→ctrl→alt→cmd order, for chord re-injection.
func heldPassthroughModifiers(state *waylandEvdevKeyState) []string {
	if state == nil {
		return nil
	}

	mods := make([]string, 0, passthroughModifierCount)

	if state.modifiers.shift > 0 {
		mods = append(mods, evdevModifierShift)
	}

	if state.modifiers.ctrl > 0 {
		mods = append(mods, evdevModifierCtrl)
	}

	if state.modifiers.alt > 0 {
		mods = append(mods, evdevModifierAlt)
	}

	if state.modifiers.cmd > 0 {
		mods = append(mods, evdevModifierCmd)
	}

	return mods
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

// newEvdevSessionState drains stale events buffered while ungrabbed and
// records which keys were already held, so the kernel's SYN_DROPPED replay
// after EVIOCGRAB is not read as fresh presses. Pre-held keys stay suppressed
// until released and pressed again.
func newEvdevSessionState(capture *waylandEvdevCapture) waylandEvdevKeyState {
	for {
		select {
		case <-capture.events:
			continue
		default:
		}

		break
	}

	pressed := make(map[uint16]bool)
	state := waylandEvdevKeyState{
		pressed:            pressed,
		initialKeys:        make(map[uint16]bool),
		releasedDuringGrab: make(map[uint16]bool),
		modifiers: evdevModifierState{
			linuxModifierState: queryEvdevModifierState(capture, pressed),
		},
	}

	queryAllPressedKeys(capture, pressed)

	for code := range pressed {
		state.initialKeys[code] = true
	}

	return state
}

// shutdownEvdevSession injects synthetic releases for keys whose real release
// never reached libinput (released during the grab, or pre-held and no longer
// pressed), releases any passthrough modifiers still held, and ungrabs.
func (et *EventTap) shutdownEvdevSession(
	capture *waylandEvdevCapture,
	state *waylandEvdevKeyState,
) {
	for code := range state.releasedDuringGrab {
		et.injectSyntheticRelease(code)
	}

	for code := range state.initialKeys {
		if !state.pressed[code] {
			et.injectSyntheticRelease(code)
		}
	}

	// A virtual modifier must never stay latched after the grab ends.
	for code, mods := range state.passthroughHeld {
		delete(state.passthroughHeld, code)
		et.releasePassthroughModifiers(mods)
	}

	capture.ungrabAll()
}

// injectSyntheticRelease posts one key-up through the virtual keyboard.
func (et *EventTap) injectSyntheticRelease(code uint16) {
	err := linux.WaylandKeyEvent(uint32(code), false)
	if err != nil && et.logger != nil {
		et.logger.Warn(
			"Failed to inject synthetic key release at shutdown",
			zap.Uint16("keycode", code),
			zap.Error(err),
		)
	}
}

// waitForEvdevKeysReleased blocks until every physically held key is released,
// or one of the two pre-grab bounds passes. Grabbing while a key is held makes
// the kernel route that key's release to our fd only — libinput never sees it,
// considers the key pressed forever, and silently eats its next press. Returns
// true when the tap was stopped while waiting.
//
// It waits in two stages, and the constants say why they are bounded
// differently. Modifiers first, because a swallowed modifier release outlives
// this mode; then everything still down, briefly, because by then it cannot be
// a modifier. Both stages end in the same place when their bound passes: the
// grab happens anyway, onto the handled path #1087 built for it.
//
// The stages are not redundant even though the second one's question subsumes
// the first — EVIOCGKEY reports modifiers like any other key, so "nothing is
// pressed" already implies "no modifier is held". What separates them is the
// bound each carries and how closely each watches: the modifier stage polls at
// five milliseconds because releasing an activation chord is on the path of
// every mode entry and the latency is paid there, while the hold stage can tick
// slower and wake on evdev traffic instead.
func (et *EventTap) waitForEvdevKeysReleased(
	capture *waylandEvdevCapture,
	overlayCapture overlaymanager.KeyboardCaptureController,
) bool {
	modifierDeadline := time.After(waylandEvdevModifierReleaseTimeout)

	for capture.modifierKeysHeld() {
		select {
		case <-et.stopCh:
			return true
		case <-modifierDeadline:
			// Grab with the modifier still down rather than never grabbing.
			// Warn, not debug: this is the keyboard being taken while the
			// kernel says a modifier is held, so the synthetic release on the
			// way out is the only thing standing between the user and a
			// modifier stuck under every application they use next.
			if et.logger != nil {
				et.logger.Warn(
					"Grabbing the keyboard with a modifier still held; its release will "+
						"not reach the compositor until this mode exits",
					zap.Duration("waited", waylandEvdevModifierReleaseTimeout),
				)
			}

			return false
		case <-time.After(waylandEvdevModifierReleasePollPeriod):
		}
	}

	held := make(map[uint16]bool)
	queryAllPressedKeys(capture, held)

	if len(held) == 0 {
		return false
	}

	if overlayCapture != nil {
		overlayCapture.SetKeyboardCaptureEnabled(true)
	}

	deadline := time.After(waylandEvdevPreGrabHoldTimeout)
	ticker := time.NewTicker(waylandEvdevPreGrabHoldPollPeriod)

	defer func() {
		ticker.Stop()

		if overlayCapture != nil {
			overlayCapture.SetKeyboardCaptureEnabled(false)
		}
	}()

	for {
		pressed := make(map[uint16]bool)
		queryAllPressedKeys(capture, pressed)

		if len(pressed) == 0 {
			return false
		}

		select {
		case <-et.stopCh:
			return true
		case <-deadline:
			// Debug rather than warn, unlike the modifier bound above: this is
			// the ordinary way a user typing into an activation gets their mode
			// promptly, and the key held through the grab costs them one
			// suppressed press. A count, never the keys — the keystream is not
			// something this may log (root AGENTS.md, Conventions).
			if et.logger != nil {
				et.logger.Debug(
					"Grabbing the keyboard with keys still held; their first press is suppressed",
					zap.Int("held", len(pressed)),
					zap.Duration("waited", waylandEvdevPreGrabHoldTimeout),
				)
			}

			return false
		case <-ticker.C:
		case _, ok := <-capture.events:
			if !ok {
				return true
			}
		}
	}
}
