//nolint:testpackage // exercises the unexported style fields through BuildStyle.
package recursivegrid

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

type mockThemeProvider struct {
	darkMode bool
}

func (m *mockThemeProvider) IsDarkMode() bool {
	return m.darkMode
}

// TestBuildStyleResolvesThemeColors pins that each color comes from the
// configured value for the active theme.
//
// It used to be a linux-only test asserting packed ARGB, because the style was
// declared per platform and Linux held it pre-converted. The style is now the
// same on every platform, so the test runs everywhere.
func TestBuildStyleResolvesThemeColors(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid

	tests := []struct {
		name            string
		dark            bool
		highlight       string
		labelBackground string
		previewText     string
	}{
		{
			name:            "light theme",
			dark:            false,
			highlight:       config.RecursiveGridHighlightColorLight,
			labelBackground: config.RecursiveGridLabelBackgroundColorLight,
			previewText:     config.RecursiveGridSubKeyPreviewTextColorLight,
		},
		{
			name:            "dark theme",
			dark:            true,
			highlight:       config.RecursiveGridHighlightColorDark,
			labelBackground: config.RecursiveGridLabelBackgroundColorDark,
			previewText:     config.RecursiveGridSubKeyPreviewTextColorDark,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			style := BuildStyle(cfg, &mockThemeProvider{darkMode: testCase.dark})

			if got := style.HighlightColor(); got != testCase.highlight {
				t.Errorf("HighlightColor() = %q, want %q", got, testCase.highlight)
			}

			if got := style.LabelBackgroundColor(); got != testCase.labelBackground {
				t.Errorf("LabelBackgroundColor() = %q, want %q", got, testCase.labelBackground)
			}

			if got := style.SubKeyPreviewTextColor(); got != testCase.previewText {
				t.Errorf("SubKeyPreviewTextColor() = %q, want %q", got, testCase.previewText)
			}
		})
	}
}

// TestBuildStyleCarriesTheToggles pins that the boolean options come from the
// configuration rather than from a hard-coded default.
//
// It sets them explicitly instead of reading DefaultConfig, because the
// defaults are platform-specific and this test now runs on every platform.
func TestBuildStyleCarriesTheToggles(t *testing.T) {
	for _, want := range []bool{true, false} {
		cfg := config.DefaultConfig().RecursiveGrid
		cfg.UI.LabelBackground = want
		cfg.UI.SubKeyPreview = want

		style := BuildStyle(cfg, &mockThemeProvider{})

		if got := style.LabelBackground(); got != want {
			t.Errorf("LabelBackground() = %v, want %v", got, want)
		}

		if got := style.SubKeyPreview(); got != want {
			t.Errorf("SubKeyPreview() = %v, want %v", got, want)
		}
	}
}

// TestStyleARGBAccessorsMatchTheHexValues pins the conversion the Cairo and GDI
// backends rely on, which used to happen inside BuildStyle.
func TestStyleARGBAccessorsMatchTheHexValues(t *testing.T) {
	style := BuildStyle(config.DefaultConfig().RecursiveGrid, &mockThemeProvider{})

	pairs := []struct {
		name string
		hex  string
		argb uint32
	}{
		{"line", style.LineColor(), style.LineColorARGB()},
		{"highlight", style.HighlightColor(), style.HighlightColorARGB()},
		{"text", style.TextColor(), style.TextColorARGB()},
		{"labelBackground", style.LabelBackgroundColor(), style.LabelBackgroundColorARGB()},
		{"previewText", style.SubKeyPreviewTextColor(), style.SubKeyPreviewTextColorARGB()},
	}

	for _, pair := range pairs {
		if want := parseHexARGB(pair.hex); pair.argb != want {
			t.Errorf("%s ARGB = %#08x, want %#08x (from %q)", pair.name, pair.argb, want, pair.hex)
		}
	}
}
