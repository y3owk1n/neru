//go:build windows

package windows

// The hook hands a chord the backend registered back to RegisterHotKey instead of
// dispatching it, so that exactly one of the two runs the binding. Which chords
// those are is compared in the normalized form both sides are matched in, and
// deleting the comparison is silent everywhere except on a Windows desktop — which
// is what this is for (ADR 0011).

import "testing"

// The chords these tests register, in the spellings a user writes.
const (
	registeredChord  = "Ctrl+G"
	otherChord       = "Ctrl+H"
	punctuationChord = "Ctrl+Alt+;"
)

func TestEventTap_IsRegisteredHotkey(t *testing.T) {
	t.Parallel()

	tap := &EventTap{}
	tap.SetHotkeys([]string{registeredChord, punctuationChord})

	registered := []string{
		registeredChord,
		// The hook reads a chord in its own spelling and casing; matching is on
		// the normalized form, so these are the same chord.
		"ctrl+g",
		"Ctrl+g",
		// Primary is the platform alias, which is Ctrl here.
		"Primary+G",
		punctuationChord,
	}

	for _, key := range registered {
		if !tap.isRegisteredHotkey(key) {
			t.Errorf("isRegisteredHotkey(%q) = false, want true: RegisterHotKey owns it", key)
		}
	}

	for _, key := range []string{otherChord, "G", "Shift+G", ""} {
		if tap.isRegisteredHotkey(key) {
			t.Errorf(
				"isRegisteredHotkey(%q) = true, want false: nothing registered it, so "+
					"handing it back would leave nothing running it",
				key,
			)
		}
	}
}

// The list is replaced, not added to, so a chord that stopped being registered
// stops being handed back.
func TestEventTap_SetHotkeys_ReplacesTheList(t *testing.T) {
	t.Parallel()

	tap := &EventTap{}
	tap.SetHotkeys([]string{registeredChord})
	tap.SetHotkeys([]string{otherChord})

	if tap.isRegisteredHotkey(registeredChord) {
		t.Error("Ctrl+G is still treated as registered after the list was replaced")
	}

	if !tap.isRegisteredHotkey(otherChord) {
		t.Error("Ctrl+H is not treated as registered after the list was replaced")
	}
}

// A sticky modifier is held by the tap itself, so the hook reads it into every
// chord. A chord that only spells a registered hotkey because of that modifier
// is the user's Ctrl+J, not their Ctrl+Shift+J, and stays with the mode.
func TestEventTap_HandleKey_StickyModifierDoesNotFireGlobalHotkey(t *testing.T) {
	t.Parallel()

	var dispatched []string

	tap := NewEventTap(func(key string) { dispatched = append(dispatched, key) }, nil)
	tap.SetHotkeys([]string{"Ctrl+Shift+J"})
	tap.notePostedModifier("shift", true)

	if !tap.handleKey("Ctrl+Shift+J", false) {
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

	// A held modifier autorepeats its key-down; one release still ends it.
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
