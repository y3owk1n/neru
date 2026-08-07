package fontgeneric_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"
)

// testFamilies stands in for a platform's three concrete families. The values
// are deliberately no platform's, so a passing test cannot be passing because
// it happened to agree with one backend's constants.
var testFamilies = fontgeneric.Families{
	Sans:  "Test Sans",
	Serif: "Test Serif",
	Mono:  "Test Mono",
}

func TestFamilies_Resolve_GenericAliases(t *testing.T) {
	// Every spelling of a generic asks for the same face: case, surrounding
	// whitespace and the separator between the words are all ignored. The
	// empty name asks for sans.
	cases := map[string]string{
		"":            testFamilies.Sans,
		"   ":         testFamilies.Sans,
		"sans":        testFamilies.Sans,
		"Sans":        testFamilies.Sans,
		"SANS":        testFamilies.Sans,
		"  sans  ":    testFamilies.Sans,
		"sansserif":   testFamilies.Sans,
		"sans serif":  testFamilies.Sans,
		"sans-serif":  testFamilies.Sans,
		"sans_serif":  testFamilies.Sans,
		"Sans Serif":  testFamilies.Sans,
		"SANS_SERIF":  testFamilies.Sans,
		"serif":       testFamilies.Serif,
		"Serif":       testFamilies.Serif,
		"  serif  ":   testFamilies.Serif,
		"mono":        testFamilies.Mono,
		"Mono":        testFamilies.Mono,
		"monospace":   testFamilies.Mono,
		"Monospace":   testFamilies.Mono,
		"mono space":  testFamilies.Mono,
		"mono-space":  testFamilies.Mono,
		"mono_space":  testFamilies.Mono,
		"   mono   ":  testFamilies.Mono,
		"MONO  SPACE": testFamilies.Mono,
	}

	for written, want := range cases {
		t.Run(written, func(t *testing.T) {
			if got := testFamilies.Resolve(written); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", written, got, want)
			}
		})
	}
}

func TestFamilies_Resolve_NamedFamilyPassesThroughTrimmed(t *testing.T) {
	// A family somebody named is handed on as written, with only the
	// surrounding whitespace removed: the platform's font system is the one
	// that knows whether it exists.
	cases := map[string]string{
		"  Arial  ":        "Arial",
		"\tJetBrains Mono": "JetBrains Mono",
		"SF Mono\n":        "SF Mono",
		// Only the outside is trimmed.
		"Helvetica  Neue": "Helvetica  Neue",
		// A generic spelling is a generic only when that is the whole name.
		"sans2":    "sans2",
		"serifish": "serifish",
	}

	for written, want := range cases {
		t.Run(written, func(t *testing.T) {
			if got := testFamilies.Resolve(written); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", written, got, want)
			}
		})
	}
}
