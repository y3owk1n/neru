//go:build windows

package windows

import "testing"

func TestWinFontResolver_GenericAliases(t *testing.T) {
	// Which spellings count as generic is pinned by the shared parser
	// (platform/fontgeneric); this pins what each generic means on Windows.
	cases := map[string]string{
		"":           defaultWindowsSans,
		"sans":       defaultWindowsSans,
		"Sans Serif": defaultWindowsSans,
		"sans_serif": defaultWindowsSans,
		"serif":      defaultWindowsSerif,
		"Monospace":  defaultWindowsMono,
		"   mono   ": defaultWindowsMono,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			fontResolver := NewFontResolver()

			if got := fontResolver.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestWinFontResolver_NamedFamilyPassesThroughTrimmed(t *testing.T) {
	// A family somebody named is handed to GDI as written, with only the
	// surrounding whitespace removed; GDI substitutes if it is missing.
	cases := map[string]string{
		"JetBrains Mono":    "JetBrains Mono",
		"  Comic Sans MS  ": "Comic Sans MS",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			fontResolver := NewFontResolver()

			if got := fontResolver.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestWinFontResolver_AnswersEachSpellingFromItsOwnName(t *testing.T) {
	// Caching must not rewrite a caller's spelling; the shared rule is pinned
	// on every CI leg in platform/fontcache, this pins that Windows uses it.
	fontResolver := NewFontResolver()

	if got := fontResolver.Resolve("Arial"); got != "Arial" {
		t.Fatalf("Resolve(%q) = %q, want %q", "Arial", got, "Arial")
	}

	if got := fontResolver.Resolve("ARIAL"); got != "ARIAL" {
		t.Fatalf("Resolve(%q) = %q, want %q", "ARIAL", got, "ARIAL")
	}
}
