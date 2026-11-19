//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

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
	// The detail must point the user at a real fix. An empty string would
	// leave them with no hint at all.
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

	body := "[General]\n"
	if scheme != "" {
		body += "ColorScheme=" + scheme + "\n"
	}

	writeKDEGlobalsBody(t, path, body)
}

func writeKDEGlobalsBody(t *testing.T, path, body string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
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
// Not parallel, because t.Setenv (HOME) is incompatible with t.Parallel.
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
			name: "dark substring in custom scheme name",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "DarkMatter")
			},
			want:   colorSchemeDark,
			wantOK: true,
		},
		{
			name: "scheme without dark substring is light",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "Custom")
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
			// Only [General] counts. A ColorScheme under another section, or
			// under one whose name starts with General, is not the scheme.
			name: "reads ColorScheme only from the General section",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobalsBody(t, primary(home), strings.Join([]string{
					"# kdeglobals fixture",
					"[KDE]",
					"ColorScheme=BreezeDark",
					"",
					"[General]",
					"ColorScheme=BreezeLight",
					"Other=value",
					"",
					"[General-Substitution]",
					"ColorScheme=OxygenDark",
				}, "\n"))
			},
			want:   colorSchemeLight,
			wantOK: true,
		},
		{
			name: "a General section without ColorScheme yields unknown",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobals(t, primary(home), "")
			},
			wantOK: false,
		},
		{
			name: "a ColorScheme outside any General section yields unknown",
			setup: func(t *testing.T, home string) {
				t.Helper()
				writeKDEGlobalsBody(t, primary(home), "[Other]\nColorScheme=BreezeDark\n")
			},
			wantOK: false,
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
