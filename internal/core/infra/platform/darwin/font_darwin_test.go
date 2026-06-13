//go:build darwin

package darwin

import "testing"

func TestMapDarwinGenericAlias_EmptyDefaultsToSans(t *testing.T) {
	if got := mapDarwinGenericAlias(""); got != defaultDarwinSans {
		t.Fatalf("expected %q for empty input, got %q", defaultDarwinSans, got)
	}
}

func TestMapDarwinGenericAlias_GenericAliases(t *testing.T) {
	cases := map[string]string{
		"":           defaultDarwinSans,
		"sans":       defaultDarwinSans,
		"Sans":       defaultDarwinSans,
		"sans-serif": defaultDarwinSans,
		"Sans Serif": defaultDarwinSans,
		"SANSSERIF":  defaultDarwinSans,
		"serif":      defaultDarwinSerif,
		"Serif":      defaultDarwinSerif,
		"mono":       defaultDarwinMono,
		"Monospace":  defaultDarwinMono,
		"   mono   ": defaultDarwinMono,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := mapDarwinGenericAlias(input); got != want {
				t.Fatalf("mapDarwinGenericAlias(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestMapDarwinGenericAlias_NonGenericPassesThrough(t *testing.T) {
	// Non-generic names are passed through to the C layer unchanged so
	// NSFontManager can verify and weight-resolve them.
	for _, input := range []string{"JetBrains Mono", "SF Mono", "Helvetica Neue"} {
		if got := mapDarwinGenericAlias(input); got != input {
			t.Fatalf("mapDarwinGenericAlias(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestNSFontResolver_CachesByFamily(t *testing.T) {
	r := &nsFontResolver{cache: make(map[string]string)}

	for range 3 {
		if got := r.Resolve("sans", true); got != defaultDarwinSans {
			t.Fatalf("expected generic alias to resolve to %q, got %q", defaultDarwinSans, got)
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.cache) != 1 {
		t.Fatalf("expected exactly one cache entry, got %d", len(r.cache))
	}
}

func TestNSFontResolver_EmptyDefaultsToSans(t *testing.T) {
	r := &nsFontResolver{cache: make(map[string]string)}

	if got := r.Resolve("", false); got != defaultDarwinSans {
		t.Fatalf("expected empty input to resolve to %q, got %q", defaultDarwinSans, got)
	}
}
