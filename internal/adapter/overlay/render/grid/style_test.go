//nolint:testpackage // exercises the unexported style fields through BuildStyle.
package grid

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
// same on every platform, so the test is too — and it runs everywhere instead
// of only in the Linux job.
func TestBuildStyleResolvesThemeColors(t *testing.T) {
	cfg := config.DefaultConfig().Grid

	tests := []struct {
		name              string
		dark              bool
		background        string
		matchedBackground string
		matchedBorder     string
		matchedText       string
	}{
		{
			name:              "light theme",
			dark:              false,
			background:        config.GridBackgroundColorLight,
			matchedBackground: config.GridMatchedBackgroundColorLight,
			matchedBorder:     config.GridMatchedBorderColorLight,
			matchedText:       config.GridMatchedTextColorLight,
		},
		{
			name:              "dark theme",
			dark:              true,
			background:        config.GridBackgroundColorDark,
			matchedBackground: config.GridMatchedBackgroundColorDark,
			matchedBorder:     config.GridMatchedBorderColorDark,
			matchedText:       config.GridMatchedTextColorDark,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			style := BuildStyle(cfg, &mockThemeProvider{darkMode: testCase.dark})

			if got := style.BackgroundColor(); got != testCase.background {
				t.Errorf("BackgroundColor() = %q, want %q", got, testCase.background)
			}

			if got := style.MatchedBackgroundColor(); got != testCase.matchedBackground {
				t.Errorf("MatchedBackgroundColor() = %q, want %q", got, testCase.matchedBackground)
			}

			if got := style.MatchedBorderColor(); got != testCase.matchedBorder {
				t.Errorf("MatchedBorderColor() = %q, want %q", got, testCase.matchedBorder)
			}

			if got := style.MatchedTextColor(); got != testCase.matchedText {
				t.Errorf("MatchedTextColor() = %q, want %q", got, testCase.matchedText)
			}
		})
	}
}

// TestStyleARGBAccessorsMatchTheHexValues pins the conversion the Cairo and GDI
// backends rely on, which used to happen inside BuildStyle.
func TestStyleARGBAccessorsMatchTheHexValues(t *testing.T) {
	style := BuildStyle(config.DefaultConfig().Grid, &mockThemeProvider{darkMode: false})

	pairs := []struct {
		name string
		hex  string
		argb uint32
	}{
		{"background", style.BackgroundColor(), style.BackgroundColorARGB()},
		{"text", style.TextColor(), style.TextColorARGB()},
		{"border", style.BorderColor(), style.LineColorARGB()},
		{"matchedText", style.MatchedTextColor(), style.MatchedTextColorARGB()},
		{"matchedBackground", style.MatchedBackgroundColor(), style.MatchedBackgroundColorARGB()},
		{"matchedBorder", style.MatchedBorderColor(), style.MatchedBorderColorARGB()},
	}

	for _, pair := range pairs {
		if want := parseHexARGB(pair.hex); pair.argb != want {
			t.Errorf("%s ARGB = %#08x, want %#08x (from %q)", pair.name, pair.argb, want, pair.hex)
		}
	}
}

// TestLineWidthKeepsAHairlineVisible pins the clamp the backends used to apply
// when they built the style.
func TestLineWidthKeepsAHairlineVisible(t *testing.T) {
	cfg := config.DefaultConfig().Grid
	cfg.UI.BorderWidth = 0

	if got := BuildStyle(cfg, &mockThemeProvider{}).LineWidth(); got < minLineWidth {
		t.Errorf("LineWidth() = %v, want at least %v", got, float64(minLineWidth))
	}
}
