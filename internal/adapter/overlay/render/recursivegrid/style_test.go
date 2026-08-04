package recursivegrid

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
)

type mockThemeProvider struct {
	darkMode bool
}

func (m *mockThemeProvider) IsDarkMode() bool {
	return m.darkMode
}

// TestBuildStyle_ResolvesThemeColors pins that each color comes from the
// configured value for the active theme.
//
// The style is one type on every platform, so this runs in every job rather
// than only where a particular backend is built.
func TestBuildStyle_ResolvesThemeColors(t *testing.T) {
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

// TestBuildStyle_CarriesTheToggles pins that the boolean options come from the
// configuration rather than from a hard-coded default.
//
// It sets them explicitly rather than reading DefaultConfig, whose values for
// these two differ by platform.
func TestBuildStyle_CarriesTheToggles(t *testing.T) {
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

// TestStyle_ARGBAccessorsMatchTheHexValues pins the conversion the Cairo and GDI
// backends rely on: the packed form of a color must be the packed form of the
// hex string beside it.
func TestStyle_ARGBAccessorsMatchTheHexValues(t *testing.T) {
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
		if want := badge.ParseHexARGB(pair.hex); pair.argb != want {
			t.Errorf("%s ARGB = %#08x, want %#08x (from %q)", pair.name, pair.argb, want, pair.hex)
		}
	}
}
