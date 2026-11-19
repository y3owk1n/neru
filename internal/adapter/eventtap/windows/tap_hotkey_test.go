//go:build windows

package windows

// The hook hands a chord the backend registered back to RegisterHotKey instead of
// dispatching it, so that exactly one of the two runs the binding. Which chords
// those are is compared in the normalized form both sides are matched in, and
// deleting the comparison is silent everywhere except on a Windows desktop,
// which is what this is for (ADR 0011).

import "testing"

// A sticky modifier is held by the tap itself, so the hook reads it into every
// chord. A chord that only spells a registered hotkey because of that modifier
// is the user's Ctrl+J, not their Ctrl+Shift+J, and stays with the mode.
func TestEventTap_HandleKey_StickyModifierDoesNotFireGlobalHotkey(t *testing.T) {
	t.Parallel()

	tap := newTestTap(t)
	tap.SetHotkeys([]string{"Ctrl+Shift+J"})
	tap.notePostedModifier("shift", true)

	dispatched, consumed := routedKey(t, tap, "Ctrl+Shift+J")

	if !consumed {
		t.Fatal("Ctrl+J with sticky Shift was handed to RegisterHotKey instead of consumed")
	}

	if len(dispatched) != 1 || dispatched[0] != "Ctrl+Shift+j" {
		t.Fatalf("dispatched = %v, want the chord handed to the mode handler", dispatched)
	}

	// The user pressing Shift as well makes it their chord again, even while
	// the sticky one is still posted.
	tap.handleKey("Shift", false)

	if tap.handleKey("Ctrl+Shift+J", false) {
		t.Fatal("physical Shift+Ctrl+J under sticky Shift was not handed to RegisterHotKey")
	}

	// A held modifier autorepeats its key-down, but one release still ends it.
	tap.handleKey("Shift", false)
	tap.handleKey("Shift", true)

	if !tap.handleKey("Ctrl+Shift+J", false) {
		t.Fatal("Ctrl+J after the physical Shift release was handed to RegisterHotKey")
	}

	// Releasing the sticky modifier restores the handoff for a physical press.
	tap.notePostedModifier("shift", false)

	if tap.handleKey("Ctrl+Shift+J", false) {
		t.Fatal("physical Ctrl+Shift+J was consumed instead of handed to RegisterHotKey")
	}
}
