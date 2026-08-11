package parity_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/parity"
)

// The words the fixtures below are built around, named once so two fixtures
// cannot disagree about how one of them is spelled.
const (
	smoothScrollEnabled = "smooth_scroll.enabled"
	hideCursorAction    = "hide_cursor"
)

func TestPlatforms_Supports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platforms parity.Platforms
		target    parity.Platform
		want      bool
	}{
		{"everywhere covers darwin", parity.AllPlatforms, parity.Darwin, true},
		{"everywhere covers windows", parity.AllPlatforms, parity.Windows, true},
		{"darwin only excludes linux", parity.Platforms{parity.Darwin}, parity.Linux, false},
		{
			"two platforms cover both",
			parity.Platforms{parity.Darwin, parity.Linux},
			parity.Linux,
			true,
		},
		{"empty column supports nothing", parity.Platforms{}, parity.Darwin, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.platforms.Supports(test.target); got != test.want {
				t.Errorf("Supports(%q) = %v, want %v", test.target, got, test.want)
			}
		})
	}
}

func TestPlatforms_Everywhere(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		platforms parity.Platforms
		want      bool
	}{
		{"every platform", parity.AllPlatforms, true},
		{"two of three", parity.Platforms{parity.Darwin, parity.Linux}, false},
		{"none", nil, false},
		{
			"out of order still everywhere",
			parity.Platforms{parity.Windows, parity.Linux, parity.Darwin},
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.platforms.Everywhere(); got != test.want {
				t.Errorf("Everywhere() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWord_Written(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word parity.Word
		want string
	}{
		{
			"option path",
			parity.Word{Kind: parity.KindOption, Name: smoothScrollEnabled},
			smoothScrollEnabled,
		},
		{
			"option value",
			parity.Word{Kind: parity.KindOption, Name: "hints.strategy", Value: "vision"},
			"hints.strategy = vision",
		},
		{
			"mode flag",
			parity.Word{Kind: parity.KindModeFlag, Name: "split-word"},
			"--split-word",
		},
		{
			"mode flag value",
			parity.Word{Kind: parity.KindModeFlag, Name: "strategy", Value: "vision"},
			"--strategy=vision",
		},
		{
			"action",
			parity.Word{Kind: parity.KindAction, Name: hideCursorAction},
			hideCursorAction,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.word.Written(); got != test.want {
				t.Errorf("Written() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPlatformFor_UnknownGOOSHasNoColumn(t *testing.T) {
	t.Parallel()

	if _, known := parity.PlatformFor("freebsd"); known {
		t.Error("PlatformFor(\"freebsd\") reported a column; only the three targets have one")
	}

	platform, known := parity.PlatformFor("linux")
	if !known || platform != parity.Linux {
		t.Errorf("PlatformFor(\"linux\") = %q, %v; want linux, true", platform, known)
	}
}

// TestDeclaration_Lookup_SeparatesAValueFromItsWord pins the reason Value is
// part of a word's identity: `hints.strategy` is recognized on every platform
// and `hints.strategy = vision` is not, so a lookup that ignored the value
// would answer one question with the other's column.
func TestDeclaration_Lookup_SeparatesAValueFromItsWord(t *testing.T) {
	t.Parallel()

	declaration := parity.Join(
		parity.Everywhere(parity.KindOption, "hints.strategy"),
		parity.ValueOn(
			parity.KindOption,
			parity.Platforms{parity.Darwin},
			"no OCR engine elsewhere",
			"vision",
			"hints.strategy",
		),
	)

	word, found := declaration.Lookup(parity.KindOption, "hints.strategy", "")
	if !found || !word.Platforms.Everywhere() {
		t.Errorf("the bare option resolved to %+v, found=%v; want every platform", word, found)
	}

	valued, found := declaration.Lookup(parity.KindOption, "hints.strategy", "vision")
	if !found || valued.Platforms.Supports(parity.Linux) {
		t.Errorf("the valued option resolved to %+v, found=%v; want darwin only", valued, found)
	}

	if _, found := declaration.Lookup(parity.KindModeFlag, "hints.strategy", ""); found {
		t.Error("an option resolved under the mode-flag kind; kind is part of a word's identity")
	}
}

func TestDeclaration_InertOn(t *testing.T) {
	t.Parallel()

	declaration := parity.Join(
		parity.Everywhere(parity.KindOption, "grid.enabled"),
		parity.On(
			parity.KindOption,
			parity.Platforms{parity.Darwin},
			"only the darwin scroll animator reads it",
			smoothScrollEnabled,
		),
		parity.On(
			parity.KindOption,
			parity.Platforms{parity.Darwin, parity.Linux},
			"not implemented on Windows",
			"smooth_cursor.steps",
		),
	)

	tests := []struct {
		name   string
		target parity.Platform
		want   []string
	}{
		{"darwin has none", parity.Darwin, nil},
		{"linux has the darwin-only one", parity.Linux, []string{smoothScrollEnabled}},
		{
			"windows has both",
			parity.Windows,
			[]string{smoothScrollEnabled, "smooth_cursor.steps"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := declaration.InertOn(test.target).Names()
			if !slices.Equal(got, test.want) {
				t.Errorf("InertOn(%q) = %v, want %v", test.target, got, test.want)
			}
		})
	}
}

func TestDeclaration_Limited_DropsTheWordsThatCarryNoSurprise(t *testing.T) {
	t.Parallel()

	declaration := parity.Join(
		parity.Everywhere(parity.KindOption, "grid.enabled", "grid.characters"),
		parity.On(
			parity.KindAction,
			parity.Platforms{parity.Darwin},
			"a Wayland client may not hide another client's cursor",
			hideCursorAction,
		),
	)

	got := declaration.Limited().Names()
	if !slices.Equal(got, []string{hideCursorAction}) {
		t.Errorf("Limited() = %v, want [hide_cursor]", got)
	}
}
