//go:build linux

package linux

import "testing"

// Tests that config-side and live-evdev-side chord spellings canonicalize equal.
// Does NOT test device reading or callback dispatch.
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

// TestCanonicalBindingSignature_DeleteNamesTheBackspaceKey pins what a
// configured "Delete" binds on the Wayland listener: the backspace key, as on
// macOS and Windows and the X11 grab. The config side folds the name, and the
// live side does not, so a press of the forward-delete key — which the
// decoder also spells "Delete" — keeps a signature no configured chord carries.
func TestCanonicalBindingSignature_DeleteNamesTheBackspaceKey(t *testing.T) {
	const wantModifiedDelete = "ctrl+shift+" + canonicalKeyBackspace

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"config delete", evdevKeyNameDelete, canonicalKeyBackspace},
		{"config delete with modifiers", "Ctrl+Shift+Delete", wantModifiedDelete},
		{"config backspace", evdevKeyNameBackspace, canonicalKeyBackspace},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalBindingSignature(tc.in); got != tc.want {
				t.Fatalf("canonicalBindingSignature(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	live := canonicalChordSignature(evdevKeyNameDelete)
	if live == canonicalBindingSignature(evdevKeyNameDelete) {
		t.Fatalf(
			"a live forward-delete press canonicalizes to %q, the signature a configured Delete binds",
			live,
		)
	}

	if got := canonicalChordSignature(evdevKeyNameBackspace); got != canonicalKeyBackspace {
		t.Fatalf("live backspace press canonicalizes to %q, want %q", got, canonicalKeyBackspace)
	}
}
