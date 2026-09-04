//go:build windows

package windows

import "testing"

const (
	arial      = "Arial"
	arialUpper = "ARIAL"
)

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

func TestWinFontResolver_InstalledFamilyResolvesToItself(t *testing.T) {
	// A family GDI has resolves to the name as written, surrounding whitespace
	// removed and case kept. Arial and Courier New ship with every Windows.
	cases := map[string]string{
		arial:         arial,
		arialUpper:    arialUpper,
		"  Arial  ":   arial,
		"Courier New": "Courier New",
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

func TestWinFontResolver_MissingFamilyFallsBackToSans(t *testing.T) {
	// A family GDI does not have lands on the sans baseline whatever face the
	// name suggests, the rule footnote 1 of docs/CROSS_PLATFORM.md states.
	cases := []string{
		"Neru Test Font That Does Not Exist",
		"Neru Missing Serif",
		"Neru Missing Mono",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			fontResolver := NewFontResolver()
			if got := fontResolver.Resolve(input); got != defaultWindowsSans {
				t.Fatalf("Resolve(%q) = %q, want %q", input, got, defaultWindowsSans)
			}
		})
	}
}

func TestWinFontResolver_AnswersEachSpellingFromItsOwnName(t *testing.T) {
	// Caching must not rewrite a caller's spelling; the shared rule is pinned
	// on every CI leg in platform/fontcache, this pins that Windows uses it.
	fontResolver := NewFontResolver()
	if got := fontResolver.Resolve(arial); got != arial {
		t.Fatalf("Resolve(%q) = %q, want %q", arial, got, arial)
	}

	if got := fontResolver.Resolve(arialUpper); got != arialUpper {
		t.Fatalf("Resolve(%q) = %q, want %q", arialUpper, got, arialUpper)
	}
}
