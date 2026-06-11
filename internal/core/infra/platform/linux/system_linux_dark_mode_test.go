//go:build linux

//nolint:testpackage // Exercises unexported helpers (scanINIValue, darkModeCapability).
package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestScanINIValueFindsKeyInCorrectSection(t *testing.T) {
	t.Parallel()

	// kdeglobals-shaped fixture: comments, multiple sections, the key we
	// care about lives only in the [General] section.
	body := strings.Join([]string{
		"# kdeglobals fixture",
		"[KDE]",
		"ColorScheme=ShouldBeIgnored",
		"",
		"[General]",
		"ColorScheme=BreezeDark",
		"Other=value",
		"",
		"[General-Substitution]",
		"ColorScheme=AlsoIgnored",
	}, "\n")

	got := scanINIValue(strings.NewReader(body), "General", "ColorScheme")
	if got != "BreezeDark" {
		t.Fatalf("scanINIValue() = %q, want %q", got, "BreezeDark")
	}
}

func TestScanINIValueReturnsEmptyWhenSectionOrKeyMissing(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing section": "[Other]\nColorScheme=BreezeDark\n",
		"missing key":     "[General]\nOther=value\n",
		"empty body":      "",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := scanINIValue(strings.NewReader(body), "General", "ColorScheme")
			if got != "" {
				t.Fatalf("scanINIValue() = %q, want empty", got)
			}
		})
	}
}

func TestDarkModeCapabilitySupportedStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value      int
		source     darkModeSource
		wantStatus ports.FeatureStatus
		wantInDtl  string
	}{
		{
			name:       "dark via portal",
			value:      colorSchemeDark,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "dark (source=xdg-portal)",
		},
		{
			name:       "light via portal",
			value:      colorSchemeLight,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "light (source=xdg-portal)",
		},
		{
			name:       "no preference via portal",
			value:      colorSchemeNoPreference,
			source:     darkModeSourcePortal,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "no preference (source=xdg-portal)",
		},
		{
			name:       "dark via kdeglobals fallback",
			value:      colorSchemeDark,
			source:     darkModeSourceKDEGlobals,
			wantStatus: ports.FeatureStatusSupported,
			wantInDtl:  "dark (source=kdeglobals)",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := darkModeCapability(testCase.value, testCase.source, true)
			if got.Status != testCase.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, testCase.wantStatus)
			}

			if !strings.Contains(got.Detail, testCase.wantInDtl) {
				t.Fatalf("Detail = %q, want substring %q", got.Detail, testCase.wantInDtl)
			}
		})
	}
}

func TestDarkModeCapabilityDowngradesToStubWhenUnreachable(t *testing.T) {
	t.Parallel()

	got := darkModeCapability(-1, "", false)

	if got.Status != ports.FeatureStatusStub {
		t.Fatalf("Status = %q, want %q", got.Status, ports.FeatureStatusStub)
	}
	// The detail must point the user at a real fix; an empty string would
	// regress to the same "we know nothing" UX as the original bug.
	if !strings.Contains(got.Detail, "xdg-desktop-portal") {
		t.Fatalf(
			"Detail = %q, want a fix-it hint mentioning xdg-desktop-portal",
			got.Detail,
		)
	}
}

// writeKDEGlobals writes a minimal kdeglobals fixture with the given
// ColorScheme into dir, creating parent directories as needed. An empty
// scheme writes a file without the key, to exercise the "present but no key"
// fall-through.
func writeKDEGlobals(t *testing.T, path, scheme string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}

	body := "[General]\n"
	if scheme != "" {
		body += "ColorScheme=" + scheme + "\n"
	}

	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

// TestReadKDEColorScheme exercises readKDEColorScheme directly through a
// fake HOME, covering the case-insensitive "dark" inference, candidate
// ordering (~/.config/kdeglobals wins over ~/.config/kdedefaults/kdeglobals),
// the no-file and no-HOME fall-throughs, and first-hit-wins semantics.
//
// Not parallel: t.Setenv (HOME) is incompatible with t.Parallel.
func TestReadKDEColorScheme(t *testing.T) {
	primary := func(home string) string {
		return filepath.Join(home, ".config", "kdeglobals")
	}
	defaults := func(home string) string {
		return filepath.Join(home, ".config", "kdedefaults", "kdeglobals")
	}

	cases := []struct {
		name   string
		setup  func(t *testing.T, home string)
		noHome bool
		want   int
		wantOK bool
	}{
		{
			name: "primary BreezeDark is dark",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "BreezeDark")
			},
			want:   colorSchemeDark,
			wantOK: true,
		},
		{
			name: "primary case-insensitive dark match",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "OXYGEN-DARK")
			},
			want:   colorSchemeDark,
			wantOK: true,
		},
		{
			name: "primary BreezeLight is light",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "BreezeLight")
			},
			want:   colorSchemeLight,
			wantOK: true,
		},
		{
			name: "falls back to kdedefaults when primary absent",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, defaults(home), "BreezeDark")
			},
			want:   colorSchemeDark,
			wantOK: true,
		},
		{
			name: "primary wins over kdedefaults (first hit)",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "BreezeLight")
				writeKDEGlobals(t, defaults(home), "BreezeDark")
			},
			want:   colorSchemeLight,
			wantOK: true,
		},
		{
			name:   "no files yields unknown",
			setup:  nil, // empty HOME dir, no kdeglobals written
			wantOK: false,
		},
		{
			name:   "no HOME yields unknown",
			noHome: true,
			wantOK: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			if testCase.noHome {
				t.Setenv("HOME", "")
			} else {
				t.Setenv("HOME", home)
			}

			if testCase.setup != nil {
				testCase.setup(t, home)
			}

			got, ok := readKDEColorScheme()
			if ok != testCase.wantOK {
				t.Fatalf("readKDEColorScheme() ok = %v, want %v", ok, testCase.wantOK)
			}

			if ok && got != testCase.want {
				t.Fatalf("readKDEColorScheme() = %d, want %d", got, testCase.want)
			}
		})
	}
}
