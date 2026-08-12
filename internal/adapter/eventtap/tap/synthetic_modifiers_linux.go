//go:build linux

package tap

import "github.com/y3owk1n/neru/internal/domain/action"

// SyntheticModifierSink is the optional extension a Tap offers when it cannot
// tell its own injected modifier keys apart from the user's without being told.
//
// It is reached by type assertion on Tap, and it is declared here — beside the
// contract it extends — rather than in whichever package injects, so a second
// Linux tap backend finds it. It is build-tagged because only X11 has the
// problem: an XTest key event carries nothing that says whose it is, so a
// modifier key Neru presses to present an action's modifiers re-enters its own
// keyboard grab as the user's, and a press followed by a release is a modifier
// tap — which with sticky_modifiers.enabled latched one nobody pressed (#1484).
// macOS stamps the modifiers on the event instead of pressing keys, and Windows
// tags every key it injects, so neither needs a channel beside the event.
//
// The caller's fallback is the behavior it had before: injecting without
// announcing. That is a live degradation rather than a failure, so a Tap that
// does not implement this still injects.
type SyntheticModifierSink interface {
	// RememberSyntheticModifier records one modifier key event the caller is
	// about to inject, so the tap reads it as Neru's rather than as the
	// user's. It names exactly one modifier — a key event resolves to one.
	//
	// Call it immediately before injecting: the tap reads on its own
	// goroutine and can see the event first otherwise.
	RememberSyntheticModifier(modifier action.Modifiers, isDown bool)
}
