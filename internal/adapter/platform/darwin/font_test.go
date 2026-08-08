//go:build darwin

package darwin

import "testing"

func TestNSFontResolver_GenericAliases(t *testing.T) {
	// Which spellings count as generic is pinned by the shared parser
	// (platform/fontgeneric); this pins what each generic means on macOS.
	cases := map[string]string{
		"":           defaultDarwinSans,
		"sans":       defaultDarwinSans,
		"Sans Serif": defaultDarwinSans,
		"sans_serif": defaultDarwinSans,
		"serif":      defaultDarwinSerif,
		"Monospace":  defaultDarwinMono,
		"   mono   ": defaultDarwinMono,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			fontResolver := NewFontResolver()

			if got := fontResolver.Resolve(input, false); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestNSFontResolver_NonGenericPassesThroughTrimmed(t *testing.T) {
	// Non-generic names are passed through to the C layer so NSFontManager
	// can verify and weight-resolve them, with only the surrounding
	// whitespace removed.
	cases := map[string]string{
		"JetBrains Mono": "JetBrains Mono",
		"SF Mono":        "SF Mono",
		"  Helvetica  ":  "Helvetica",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			fontResolver := NewFontResolver()

			if got := fontResolver.Resolve(input, false); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestNSFontResolver_AnswersEachSpellingFromItsOwnName(t *testing.T) {
	// Caching must not rewrite a caller's spelling; the shared rule is pinned
	// on every CI leg in platform/fontcache, this pins that macOS uses it.
	fontResolver := NewFontResolver()

	if got := fontResolver.Resolve("Arial", false); got != "Arial" {
		t.Fatalf("Resolve(%q) = %q, want %q", "Arial", got, "Arial")
	}

	if got := fontResolver.Resolve("ARIAL", false); got != "ARIAL" {
		t.Fatalf("Resolve(%q) = %q, want %q", "ARIAL", got, "ARIAL")
	}
}
