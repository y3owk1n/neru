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
			fontResolver := &nsFontResolver{cache: make(map[string]string)}

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
			fontResolver := &nsFontResolver{cache: make(map[string]string)}

			if got := fontResolver.Resolve(input, false); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestNSFontResolver_CachesByFamily(t *testing.T) {
	fontResolver := &nsFontResolver{cache: make(map[string]string)}

	for range 3 {
		if got := fontResolver.Resolve("sans", true); got != defaultDarwinSans {
			t.Fatalf("expected generic alias to resolve to %q, got %q", defaultDarwinSans, got)
		}
	}

	fontResolver.mu.RLock()
	defer fontResolver.mu.RUnlock()

	if len(fontResolver.cache) != 1 {
		t.Fatalf("expected exactly one cache entry, got %d", len(fontResolver.cache))
	}
}
