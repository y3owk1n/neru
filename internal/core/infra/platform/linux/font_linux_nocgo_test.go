//go:build linux && !cgo

package linux

import "testing"

func TestMapGenericAlias_EmptyDefaultsToSans(t *testing.T) {
	if got := mapGenericAlias(""); got != defaultLinuxSans {
		t.Fatalf("expected %q for empty input, got %q", defaultLinuxSans, got)
	}
}

func TestMapGenericAlias_GenericAliases(t *testing.T) {
	cases := map[string]string{
		"sans":       defaultLinuxSans,
		"Sans":       defaultLinuxSans,
		"SANS":       defaultLinuxSans,
		"sans-serif": defaultLinuxSans,
		"Sans Serif": defaultLinuxSans,
		"SANSSERIF":  defaultLinuxSans,
		"serif":      defaultLinuxSerif,
		"Serif":      defaultLinuxSerif,
		"mono":       defaultLinuxMono,
		"Monospace":  defaultLinuxMono,
		"   mono   ": defaultLinuxMono,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := mapGenericAlias(input); got != want {
				t.Fatalf("mapGenericAlias(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestMapGenericAlias_NonGenericPassesThrough(t *testing.T) {
	// Non-generic names are returned unchanged (trimmed) so the CGO path
	// can ask fontconfig to verify them.
	for _, input := range []string{"JetBrains Mono", "DejaVu Sans", "Comic Sans MS"} {
		if got := mapGenericAlias(input); got != input {
			t.Fatalf("mapGenericAlias(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestPassthroughResolver_CachesByFamily(t *testing.T) {
	r := passthroughResolver{}

	// Repeated calls with the same input should not mutate behaviour.
	for range 3 {
		if got := r.Resolve("sans", true); got != defaultLinuxSans {
			t.Fatalf("expected generic alias to resolve to %q, got %q", defaultLinuxSans, got)
		}
	}
}

func TestPassthroughResolver_EmptyDefaultsToSans(t *testing.T) {
	r := passthroughResolver{}

	if got := r.Resolve("", false); got != defaultLinuxSans {
		t.Fatalf("expected empty input to resolve to %q, got %q", defaultLinuxSans, got)
	}
}
