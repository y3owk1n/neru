//go:build linux

package tap

import "github.com/y3owk1n/neru/internal/domain/action"

// SyntheticModifierSink is the optional extension a Tap offers when it cannot
// tell its own injected modifier keys apart from the user's without being told.
//
// It is reached by type assertion on Tap and declared beside the contract it
// extends, so a second Linux tap backend finds it. Only X11 needs it, because
// an XTest key event carries no origin, so a modifier key Neru presses
// re-enters its own keyboard grab as the user's, and with
// sticky_modifiers.enabled a press plus release latched a modifier nobody
// pressed (#1484). macOS stamps modifiers on the event and Windows tags
// injected keys, so neither needs this.
//
// A Tap that does not implement this still injects, unannounced. That is a
// degradation, not a failure.
type SyntheticModifierSink interface {
	// RememberSyntheticModifier records one modifier key event the caller is
	// about to inject, so the tap reads it as Neru's rather than as the
	// user's. It names exactly one modifier. A key event resolves to one.
	//
	// Call it immediately before injecting. The tap reads on its own
	// goroutine and can see the event first otherwise.
	RememberSyntheticModifier(modifier action.Modifiers, isDown bool)
}
