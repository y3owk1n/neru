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
