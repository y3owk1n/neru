//go:build darwin

package darwin

import "testing"

func TestNSFontResolver_GenericAliases(t *testing.T) {
	// Which spellings count as generic is pinned by the shared parser
	// (platform/fontgeneric). This pins what each generic means on macOS.
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

			if got := fontResolver.Resolve(input); got != want {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
