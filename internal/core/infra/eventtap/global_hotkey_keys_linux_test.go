//go:build linux

// internal/core/infra/eventtap/global_hotkey_keys_linux_test.go
// Tests that config-side and live-evdev-side chord spellings canonicalize equal.
// Does NOT test device reading or callback dispatch.

package eventtap //nolint:testpackage // white-box: exercises the unexported canonicalChordSignature.

import "testing"

const wantGridChord = "ctrl+shift+g"

func TestCanonicalChordSignature(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"config grid", "Ctrl+Shift+G", wantGridChord},
		{"live grid (evdev order)", "Shift+Ctrl+g", wantGridChord},
		{"config hints space", "Ctrl+Shift+Space", "ctrl+shift+space"},
		{"live hints space", "Shift+Ctrl+Space", "ctrl+shift+space"},
		{"primary alias", "Primary+Shift+C", "ctrl+shift+c"},
		{"super alias", "Super+L", "cmd+l"},
		{"dedupe + trim", " ctrl + Ctrl + shift + g ", wantGridChord},
		{"bare key", evdevKeyNameEscape, "escape"},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalChordSignature(tc.in); got != tc.want {
				t.Fatalf("canonicalChordSignature(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCanonicalChordSignatureMatchesAcrossSides(t *testing.T) {
	// The config registers "Ctrl+Shift+G"; the evdev decoder emits
	// "Shift+Ctrl+g". Both must resolve to the same signature or the hotkey
	// never fires.
	if canonicalChordSignature("Ctrl+Shift+G") != canonicalChordSignature("Shift+Ctrl+g") {
		t.Fatal("config and live spellings of Ctrl+Shift+G do not match")
	}
}
