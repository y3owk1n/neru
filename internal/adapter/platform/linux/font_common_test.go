//go:build linux

package linux

import "testing"

func TestLinuxFamilies_Resolve_GenericAliases(t *testing.T) {
	// Which spellings count as generic is pinned by the shared parser
	// (platform/fontgeneric); this pins what each generic means on Linux.
	cases := map[string]string{
		"":           defaultLinuxSans,
		"sans":       defaultLinuxSans,
		"Sans Serif": defaultLinuxSans,
		"sans_serif": defaultLinuxSans,
		"serif":      defaultLinuxSerif,
		"Monospace":  defaultLinuxMono,
		"   mono   ": defaultLinuxMono,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := linuxFamilies.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestLinuxFamilies_Resolve_NamedFamilyPassesThroughTrimmed(t *testing.T) {
	// Non-generic names are handed to fontconfig to verify, with only the
	// surrounding whitespace removed.
	cases := map[string]string{
		"JetBrains Mono": "JetBrains Mono",
		"Comic Sans MS":  "Comic Sans MS",
		"  DejaVu Sans ": "DejaVu Sans",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := linuxFamilies.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestDefaultForMapped_KeepsTheBaselineThatWasMapped(t *testing.T) {
	// When fontconfig says the family is missing, a mapped baseline is kept and
	// anything else falls back to sans. The input is a family already mapped,
	// not what the user wrote.
	cases := map[string]string{
		defaultLinuxSans:  defaultLinuxSans,
		defaultLinuxSerif: defaultLinuxSerif,
		defaultLinuxMono:  defaultLinuxMono,
		"Iosevka":         defaultLinuxSans,
		"":                defaultLinuxSans,
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := defaultForMapped(input); got != want {
				t.Fatalf("defaultForMapped(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
