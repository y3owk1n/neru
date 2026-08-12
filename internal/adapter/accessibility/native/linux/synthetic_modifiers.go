//go:build linux

package linux

import (
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// The sink is a process-global slot for the same reason the config provider
// beside it is: this package's injection entry points are package-level
// functions the dispatch table in ../backend_linux.go names, with nowhere to
// hang a dependency. The interface itself lives beside the tap contract it
// extends (tap.SyntheticModifierSink); what is set here is the live tap, wired
// once at daemon startup and let go of at teardown — see
// internal/app/wiring_platform_linux.go.
//
// A nil sink means "nobody is listening", which is what the CLI, the tests, and
// every moment of startup before the phase that builds a tap are in.
var (
	syntheticModifierMu   sync.RWMutex
	syntheticModifierSink tap.SyntheticModifierSink
)

// SetSyntheticModifierSink wires the live event tap into the injection path, so
// the modifier keys an action presents are announced before they are pressed.
// Passing nil unwires it.
func SetSyntheticModifierSink(sink tap.SyntheticModifierSink) {
	syntheticModifierMu.Lock()
	syntheticModifierSink = sink
	syntheticModifierMu.Unlock()
}

// recordSyntheticModifier announces one modifier key event to the wired sink.
//
// With no sink wired the injection still happens, unannounced — which is the
// behavior this seam improves on rather than a reason to withhold the key the
// caller asked for.
func recordSyntheticModifier(modifier action.Modifiers, isDown bool) {
	syntheticModifierMu.RLock()

	sink := syntheticModifierSink

	syntheticModifierMu.RUnlock()

	if sink == nil {
		return
	}

	sink.RememberSyntheticModifier(modifier, isDown)
}
