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
			fontResolver := &winFontResolver{cache: make(map[string]string)}

			if got := fontResolver.Resolve(input, false); got != want {
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
			fontResolver := &winFontResolver{cache: make(map[string]string)}

			if got := fontResolver.Resolve(input, false); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestWinFontResolver_CachesByFamily(t *testing.T) {
	fontResolver := &winFontResolver{cache: make(map[string]string)}

	for range 3 {
		if got := fontResolver.Resolve("sans", true); got != defaultWindowsSans {
			t.Fatalf("expected generic alias to resolve to %q, got %q", defaultWindowsSans, got)
		}
	}

	fontResolver.mu.RLock()
	defer fontResolver.mu.RUnlock()

	if len(fontResolver.cache) != 1 {
		t.Fatalf("expected exactly one cache entry, got %d", len(fontResolver.cache))
	}
}
