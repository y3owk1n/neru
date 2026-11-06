// A mode's session on the evdev proxy: what happens to a key the proxy
// withheld for it, and what the tap does to open and close one.

//go:build linux && cgo

package linux

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// evdevSession is the consumer of key events while a mode is open. Every
// method runs on the proxy's run goroutine.
//
// It counts only modifier presses made since it began. Every modifier press
// is forwarded to the compositor and the session reads it on the way past, as
// the macOS tap reads flags off an event it returns. A modifier already down
// at the start belongs to the activation chord. It is never a sticky toggle
// but still names chords, so Alt held from the chord plus Space is Alt+Space
// on every platform. Sticky detection arms once the chord is fully up.
type evdevSession struct {
	tap   *EventTap
	proxy *evdevProxy

	state waylandEvdevKeyState

	// dispatchedDown marks keys whose press reached the mode, so the mode only
	// gets releases of presses it saw.
	dispatchedDown map[uint16]bool

	// forwardedModifiers is the activation chord's modifiers still down when
	// the session began. Sticky detection arms once they are all up.
	forwardedModifiers map[uint16]bool

	// ended is closed when the proxy detaches the session itself, on failing
	// open. The keyboards reach the compositor again, so the tap falls back to
	// the overlay's keyboard focus.
	ended chan struct{}
}

func newEvdevSession(tap *EventTap) *evdevSession {
	return &evdevSession{
		tap:                tap,
		state:              newWaylandEvdevKeyState(),
		dispatchedDown:     make(map[uint16]bool),
		forwardedModifiers: make(map[uint16]bool),
		ended:              make(chan struct{}),
	}
}

// begin runs on the run goroutine as the session takes over, and reads the
// proxy's picture of the keyboard for the modifiers already forwarded.
func (s *evdevSession) begin(proxy *evdevProxy) {
	s.proxy = proxy

	for code := range proxy.global.pressed {
		if proxy.capture.modifierName(code) != "" && proxy.rule.isDown(code) {
			s.forwardedModifiers[code] = true
		}
	}

	s.armIfClear()
}

func (s *evdevSession) handlePress(code uint16, modifier string, forwarded bool) {
	if modifier != "" {
		// Forwarded, like every modifier. The session reads it unless it
		// is the activation chord's, on this keyboard or a second one.
		if s.forwardedModifiers[code] {
			return
		}

		s.state.trackModifier(code, modifier, true)
		s.dispatchStickyToggle(modifier, true)

		return
	}

	if forwarded {
		// A second keyboard repeating a key the compositor already has down
		// is not the mode's.
		return
	}

	chord := s.chordName(code)
	if chord == "" {
		return
	}

	// Modifier passthrough: an unbound Ctrl/Alt/Cmd chord goes out on the
	// proxy keyboard and stays the compositor's until released.
	if s.tap.shouldPassthroughChord(chord) {
		s.proxy.forwardWithheld(code)
		s.tap.firePassthroughCallback()

		return
	}

	s.dispatchedDown[code] = true
	s.tap.dispatchKey(chord)
}

func (s *evdevSession) handleRepeat(code uint16, modifier string, forwarded bool) {
	if forwarded || modifier != "" || !s.dispatchedDown[code] {
		return
	}

	chord := s.chordName(code)
	if chord == "" {
		return
	}

	s.tap.dispatchKey(chord)
}

func (s *evdevSession) handleRelease(code uint16, modifier string, forwarded bool) {
	if modifier != "" {
		if s.forwardedModifiers[code] {
			// The compositor sees the key up only when the last keyboard
			// holding it lets go, the release the rule forwards.
			if forwarded {
				delete(s.forwardedModifiers, code)
				s.armIfClear()
			}

			return
		}

		if s.state.trackModifier(code, modifier, false) == modifierReleaseUnmatched {
			return
		}

		s.dispatchStickyToggle(modifier, false)
		s.armIfClear()

		return
	}

	if !s.dispatchedDown[code] {
		return
	}

	delete(s.dispatchedDown, code)

	if key := s.proxy.capture.keyName(code); key != "" {
		if keyUp := keyvocab.KeyUpEvent(key); keyUp != "" {
			s.tap.dispatchKey(keyUp)
		}
	}
}

func (s *evdevSession) chordName(code uint16) string {
	key := s.proxy.capture.keyName(code)
	if key == "" {
		return ""
	}

	held := s.state.modifiers.linuxModifierState
	for forwarded := range s.forwardedModifiers {
		held.update(s.proxy.capture.modifierName(forwarded), true)
	}

	return keyvocab.NormalizeKey(held.prefix() + key)
}

func (s *evdevSession) dispatchStickyToggle(modifier string, isDown bool) {
	if s.tap.stickyToggleEnabled() && s.tap.stickyDetectionArmed() {
		s.tap.dispatchKey(keyvocab.ModifierToggleEvent(modifier, isDown))
	}
}

// armIfClear arms sticky detection once every modifier down was seen pressed
// by the mode, so activation-chord releases are not sticky toggles, as on
// macOS.
func (s *evdevSession) armIfClear() {
	if s.tap.stickyDetectionArmed() {
		return
	}

	if len(s.forwardedModifiers) == 0 && s.state.modifiers.allZero() {
		s.tap.stickyArmDetection()
	}
}

// runWaylandEvdev captures keys for the mode through the proxy until the tap is
// disabled. It reports false when the proxy cannot serve a session, so the
// caller falls back to the overlay's keyboard focus.
func (et *EventTap) runWaylandEvdev() bool {
	proxy, err := acquireEvdevProxy(et.logger)
	if err != nil {
		if et.logger != nil {
			et.logger.Info(
				"Wayland evdev capture unavailable; falling back to overlay keyboard focus",
				zap.Error(err),
			)
		}

		return false
	}

	session := newEvdevSession(et)

	err = proxy.startSession(session)
	if err != nil {
		if et.logger != nil {
			et.logger.Warn(
				"Evdev proxy cannot capture keys; falling back to overlay keyboard focus",
				zap.Error(err),
			)
		}

		return false
	}

	// The proxy owns the keyboard, so the overlay stays keyboard-passive (see
	// waylandEvdevKeyboardActive). Set once, on the first session.
	et.overlayPassiveOnce.Do(func() {
		if controller, ok := overlay.Get().(overlaymanager.KeyboardCaptureController); ok {
			controller.SetKeyboardCaptureEnabled(false)
		}
	})

	if et.logger != nil {
		et.logger.Debug(
			"Using Wayland evdev keyboard capture",
			zap.Int("devices", proxy.deviceCount()),
		)
	}

	select {
	case <-et.stopCh:
		proxy.stopSession()

		return true
	case <-session.ended:
		if et.logger != nil {
			et.logger.Warn(
				"Evdev proxy released the keyboards mid-mode; falling back to overlay keyboard focus",
			)
		}

		return false
	}
}
