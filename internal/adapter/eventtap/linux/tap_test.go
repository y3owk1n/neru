//go:build linux

package linux

import "testing"

const (
	chordCtrlC     = "Ctrl+c"
	chordShiftCtrl = "shift+ctrl+t"
)

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
