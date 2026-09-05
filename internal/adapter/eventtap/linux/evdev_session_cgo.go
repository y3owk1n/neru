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
// It keeps its own picture of the modifiers, counting only presses the proxy
// withheld for it. A modifier that was already down when the session began —
// the activation chord's — was forwarded, so the compositor owns its release,
// and the mode never sees it: a hint label typed while Super is still coming
// up is the label, not Super plus the label. That is also what re-arms sticky
// detection cleanly: the session starts at a clean slate of its own and waits
// only for the chord to finish coming up.
type evdevSession struct {
	tap   *EventTap
	proxy *evdevProxy

	state waylandEvdevKeyState

	// dispatchedDown marks keys whose press reached the mode, so the release
	// that reaches it is only ever one it saw the press of.
	dispatchedDown map[uint16]bool

	// forwardedModifiers is the activation chord's modifiers still down when
	// the session began: sticky detection arms once they are all up.
	forwardedModifiers map[uint16]bool
}

func newEvdevSession(tap *EventTap) *evdevSession {
	return &evdevSession{
		tap:                tap,
		state:              newWaylandEvdevKeyState(),
		dispatchedDown:     make(map[uint16]bool),
		forwardedModifiers: make(map[uint16]bool),
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
	if forwarded {
		// Held before the session, or a second keyboard repeating a key the
		// compositor already has down: not the mode's.
		return
	}

	if modifier != "" {
		s.state.trackModifier(code, modifier, true)
		s.dispatchStickyToggle(modifier, true)

		return
	}

	chord := s.chordName(code)
	if chord == "" {
		return
	}

	// Modifier passthrough: an unbound Ctrl/Alt/Cmd chord the focused app
	// should get goes out on the proxy keyboard after all, and stays the
	// compositor's until it is released.
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

func (s *evdevSession) handleRelease(code uint16, modifier string, _ bool) {
	if modifier != "" {
		if s.forwardedModifiers[code] {
			delete(s.forwardedModifiers, code)
			s.armIfClear()

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

	return keyvocab.NormalizeKey(s.state.modifiers.prefix() + key)
}

func (s *evdevSession) dispatchStickyToggle(modifier string, isDown bool) {
	if s.tap.stickyToggleEnabled() && s.tap.stickyDetectionArmed() {
		s.tap.dispatchKey(keyvocab.ModifierToggleEvent(modifier, isDown))
	}
}

// armIfClear arms sticky detection once no modifier is down that the mode
// did not see pressed, matching macOS, where the activation chord's modifier
// releases are not read as sticky toggles.
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

	err = proxy.startSession(newEvdevSession(et))
	if err != nil {
		if et.logger != nil {
			et.logger.Warn(
				"Evdev proxy cannot capture keys; falling back to overlay keyboard focus",
				zap.Error(err),
			)
		}

		return false
	}

	waylandEvdevKeyboardActive.Store(true)
	defer waylandEvdevKeyboardActive.Store(false)

	// The proxy owns the keyboard, so the overlay stays keyboard-passive: a
	// layer-surface grab would only deactivate the focused app's toplevel on
	// wlroots. Said once, on the first session; the overlay keeps it.
	et.overlayPassiveOnce.Do(func() {
		if controller, ok := overlay.Get().(overlaymanager.KeyboardCaptureController); ok {
			controller.SetKeyboardCaptureEnabled(false)
		}
	})

	if et.logger != nil {
		et.logger.Info(
			"Using Wayland evdev keyboard capture",
			zap.Int("devices", proxy.deviceCount()),
		)
	}

	<-et.stopCh

	proxy.stopSession()

	return true
}
