//go:build linux

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	chordCtrlC     = "Ctrl+c"
	chordShiftCtrl = "shift+ctrl+t"
)

// TestEventTap_RememberSyntheticModifier_MakesAnInjectedTapConsumable is the
// #1484 fix from the tap's side.
//
// An XTest key event re-enters the X11 grab indistinguishable from the user's
// own, so a press followed by a release of the same key *is* a modifier tap by
// the tap's own definition, and with sticky_modifiers.enabled that latches a
// modifier the user never touched. The tap already knows how to disown an
// event it is about to inject; this is the entry point that lets the
// accessibility injection path — which posts its own key events, not through
// PostModifierEvent — say so too.
func TestEventTap_RememberSyntheticModifier_MakesAnInjectedTapConsumable(t *testing.T) {
	t.Parallel()

	eventTap := NewEventTap(nil, nil)

	eventTap.RememberSyntheticModifier(action.ModCtrl, true)
	eventTap.RememberSyntheticModifier(action.ModCtrl, false)

	if !eventTap.consumeSyntheticModifierEvent(evdevModifierCtrl, true) {
		t.Fatal("the injected ctrl press was not consumed, so the tap counts it as the user's")
	}

	if !eventTap.consumeSyntheticModifierEvent(evdevModifierCtrl, false) {
		t.Fatal("the injected ctrl release was not consumed, completing a false modifier tap")
	}
}

// TestEventTap_RememberSyntheticModifier_LeavesAModifierNobodyInjectedAlone is
// the other half: the suppression window is 250ms wide and shared, so
// registering more than what is actually injected would swallow a real
// modifier press the user made inside it.
func TestEventTap_RememberSyntheticModifier_LeavesAModifierNobodyInjectedAlone(t *testing.T) {
	t.Parallel()

	eventTap := NewEventTap(nil, nil)

	eventTap.RememberSyntheticModifier(action.ModCtrl, true)

	if eventTap.consumeSyntheticModifierEvent(evdevModifierShift, true) {
		t.Fatal("a shift press nothing injected was consumed")
	}

	if eventTap.consumeSyntheticModifierEvent(evdevModifierCtrl, false) {
		t.Fatal("a ctrl release nothing injected was consumed")
	}
}

// TestEventTap_RememberSyntheticModifier_RegistersOneNamePerKeyEvent pins the
// translation from the injection path's vocabulary to the tap's, and that it is
// one-for-one: the grab reads a key event under exactly one name, and consuming
// it takes one entry off the queue. A second entry nothing answers would sit
// out the whole suppression window and swallow a modifier the user pressed.
func TestEventTap_RememberSyntheticModifier_RegistersOneNamePerKeyEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifiers action.Modifiers
		want      []string
	}{
		{
			name:      "cmd",
			modifiers: action.ModCmd,
			want:      []string{evdevModifierCmd},
		},
		{
			name:      "shift",
			modifiers: action.ModShift,
			want:      []string{evdevModifierShift},
		},
		{
			name:      "alt",
			modifiers: action.ModAlt,
			want:      []string{evdevModifierAlt},
		},
		{
			name:      "ctrl",
			modifiers: action.ModCtrl,
			want:      []string{evdevModifierCtrl},
		},
		{
			// One keycode under two modifier names is what an X11 layout
			// resolving Meta onto the Super key produces. The grab still reads
			// its key event under one of them, so announcing both would leave
			// the other unanswered.
			name:      "a set naming two modifiers",
			modifiers: action.ModAlt | action.ModCmd,
			want:      nil,
		},
		{name: "nothing to announce", modifiers: 0, want: nil},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			eventTap := NewEventTap(nil, nil)
			eventTap.RememberSyntheticModifier(testCase.modifiers, true)

			for _, modifier := range testCase.want {
				if !eventTap.consumeSyntheticModifierEvent(modifier, true) {
					t.Fatalf("RememberSyntheticModifier(%s) did not register %q",
						testCase.modifiers, modifier)
				}
			}

			everyModifier := []string{
				evdevModifierCmd,
				evdevModifierShift,
				evdevModifierAlt,
				evdevModifierCtrl,
			}
			for _, modifier := range everyModifier {
				if eventTap.consumeSyntheticModifierEvent(modifier, true) {
					t.Fatalf("RememberSyntheticModifier(%s) registered a stray %q",
						testCase.modifiers, modifier)
				}
			}
		})
	}
}

func TestSyntheticModifierSuppressionConsumesOnce(t *testing.T) {
	t.Parallel()

	eventTap := NewEventTap(nil, nil)
	eventTap.rememberSyntheticModifierEvent("shift", true)

	if !eventTap.consumeSyntheticModifierEvent("shift", true) {
		t.Fatal("expected first matching synthetic event to be consumed")
	}

	if eventTap.consumeSyntheticModifierEvent("shift", true) {
		t.Fatal("expected synthetic event to be consumed only once")
	}
}

func TestCanonicalChordForMatchOrderIndependent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single ctrl lowercased", in: chordCtrlC, want: "ctrl+c"},
		{name: "cmd cased", in: "Cmd+C", want: "cmd+c"},
		{name: "ctrl+shift order a", in: "ctrl+shift+t", want: chordShiftCtrl},
		{name: "ctrl+shift order b", in: chordShiftCtrl, want: chordShiftCtrl},
		{name: "all mods reordered", in: "cmd+alt+ctrl+shift+k", want: "shift+ctrl+alt+cmd+k"},
		{name: "control alias", in: "control+a", want: "ctrl+a"},
		{name: "plain key", in: "c", want: "c"},
		{name: "empty", in: "", want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := canonicalChordForMatch(testCase.in); got != testCase.want {
				t.Fatalf(
					"canonicalChordForMatch(%q) = %q, want %q",
					testCase.in,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestShouldPassthroughChord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		enabled     bool
		blacklist   []string
		intercepted []string
		chord       string
		want        bool
	}{
		{name: "disabled", enabled: false, chord: chordCtrlC, want: false},
		{name: "unbound ctrl passes", enabled: true, chord: chordCtrlC, want: true},
		{name: "unbound alt passes", enabled: true, chord: "Alt+Left", want: true},
		{name: "shift only excluded", enabled: true, chord: "Shift+a", want: false},
		{name: "plain key excluded", enabled: true, chord: "c", want: false},
		{
			name:      "blacklisted consumed",
			enabled:   true,
			blacklist: []string{chordCtrlC},
			chord:     chordCtrlC,
			want:      false,
		},
		{
			name:      "blacklist order independent",
			enabled:   true,
			blacklist: []string{"Cmd+Shift+K"},
			chord:     "Shift+Cmd+k",
			want:      false,
		},
		{
			name:        "intercepted consumed",
			enabled:     true,
			intercepted: []string{"Ctrl+j"},
			chord:       "Ctrl+j",
			want:        false,
		},
		{
			name:      "non-blacklisted sibling passes",
			enabled:   true,
			blacklist: []string{chordCtrlC},
			chord:     "Ctrl+v",
			want:      true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			eventTap := NewEventTap(nil, nil)
			eventTap.SetModifierPassthrough(testCase.enabled, testCase.blacklist)
			eventTap.SetInterceptedModifierKeys(testCase.intercepted)

			if got := eventTap.shouldPassthroughChord(testCase.chord); got != testCase.want {
				t.Fatalf(
					"shouldPassthroughChord(%q) = %t, want %t",
					testCase.chord,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestSetModifierPassthroughDisabledStillMatchesBlacklist(t *testing.T) {
	t.Parallel()

	// Even with the blacklist populated, a disabled passthrough never passes
	// anything through — the enabled flag gates first.
	eventTap := NewEventTap(nil, nil)
	eventTap.SetModifierPassthrough(false, []string{"Ctrl+x"})

	if eventTap.shouldPassthroughChord("Ctrl+y") {
		t.Fatal("expected no passthrough while disabled")
	}
}
