//go:build linux

package linux

import (
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// The sink is a process-global slot, like the logger slot beside it,
// because the injection entry points are package-level functions named by
// ../backend_linux.go. The interface is tap.SyntheticModifierSink. The live
// tap is wired at daemon startup and cleared at teardown (see
// internal/app/wiring_platform_linux.go). A nil sink means nobody is
// listening, as in the CLI, tests, and startup before the tap is built.
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
// With no sink the injection still happens, unannounced.
func recordSyntheticModifier(modifier action.Modifiers, isDown bool) {
	syntheticModifierMu.RLock()

	sink := syntheticModifierSink

	syntheticModifierMu.RUnlock()

	if sink == nil {
		return
	}

	sink.RememberSyntheticModifier(modifier, isDown)
}
