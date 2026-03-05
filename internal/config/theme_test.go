package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// mockThemeProvider implements config.ThemeProvider for testing.
type mockThemeProvider struct {
	darkMode bool
}

func (m *mockThemeProvider) IsDarkMode() bool {
	return m.darkMode
}

func TestResolveColor_UserSpecified(t *testing.T) {
	// When user explicitly sets colors, they should be used.
	lightColor := "#111111"
	darkColor := "#EEEEEE"
	defaultLight := "#AAAAAA"
	defaultDark := "#BBBBBB"

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks dark color
	result := config.ResolveColor(lightColor, darkColor, darkTheme, defaultLight, defaultDark)
	if result != darkColor {
		t.Errorf("Expected dark color %q, got %q", darkColor, result)
	}

	// Light mode picks light color
	result = config.ResolveColor(lightColor, darkColor, lightTheme, defaultLight, defaultDark)
	if result != lightColor {
		t.Errorf("Expected light color %q, got %q", lightColor, result)
	}

	// Nil theme picks light color
	result = config.ResolveColor(lightColor, darkColor, nil, defaultLight, defaultDark)
	if result != lightColor {
		t.Errorf("Expected light color fallback %q, got %q", lightColor, result)
	}
}

func TestResolveColor_ThemeAwareDefaults(t *testing.T) {
	// When both light and dark colors are empty, use provided defaults.
	defaultLight := "#AAAAAA"
	defaultDark := "#BBBBBB"

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks default dark
	result := config.ResolveColor("", "", darkTheme, defaultLight, defaultDark)
	if result != defaultDark {
		t.Errorf("Expected default dark %q, got %q", defaultDark, result)
	}

	// Light mode picks default light
	result = config.ResolveColor("", "", lightTheme, defaultLight, defaultDark)
	if result != defaultLight {
		t.Errorf("Expected default light %q, got %q", defaultLight, result)
	}
}

func TestResolveColor_RecursiveGridText(t *testing.T) {
	// When text color is empty (theme-aware default), it should use the
	// RecursiveGridTextColor constants.
	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks RecursiveGridTextColorDark (white)
	result := config.ResolveColor(
		"",
		"",
		darkTheme,
		config.RecursiveGridTextColorLight,
		config.RecursiveGridTextColorDark,
	)
	if result != config.RecursiveGridTextColorDark {
		t.Errorf(
			"Expected dark mode color %q, got %q",
			config.RecursiveGridTextColorDark,
			result,
		)
	}

	// Light mode picks RecursiveGridTextColorLight (black)
	result = config.ResolveColor(
		"",
		"",
		lightTheme,
		config.RecursiveGridTextColorLight,
		config.RecursiveGridTextColorDark,
	)
	if result != config.RecursiveGridTextColorLight {
		t.Errorf(
			"Expected light mode color %q, got %q",
			config.RecursiveGridTextColorLight,
			result,
		)
	}

	// Nil theme provider picks light mode color
	result = config.ResolveColor(
		"",
		"",
		nil,
		config.RecursiveGridTextColorLight,
		config.RecursiveGridTextColorDark,
	)
	if result != config.RecursiveGridTextColorLight {
		t.Errorf(
			"Expected light mode fallback %q, got %q",
			config.RecursiveGridTextColorLight,
			result,
		)
	}
}

func TestDefaultConfig_ColorsArePopulated(t *testing.T) {
	// Verify that the default config now contains explicit color values
	// instead of empty strings.
	cfg := config.DefaultConfig()

	if cfg.RecursiveGrid.TextColorLight != config.RecursiveGridTextColorLight {
		t.Errorf(
			"Expected RecursiveGridTextColorLight %q in default config, got %q",
			config.RecursiveGridTextColorLight,
			cfg.RecursiveGrid.TextColorLight,
		)
	}

	if cfg.Hints.BackgroundColorLight != config.HintsBackgroundColorLight {
		t.Errorf(
			"Expected HintsBackgroundColorLight %q in default config, got %q",
			config.HintsBackgroundColorLight,
			cfg.Hints.BackgroundColorLight,
		)
	}
}

func TestRecursiveGridTextColorConstants(t *testing.T) {
	// Verify the theme color constants are valid hex colors.
	if config.RecursiveGridTextColorDark != "#FFFFFFFF" {
		t.Errorf(
			"Expected RecursiveGridTextColorDark to be #FFFFFFFF, got %q",
			config.RecursiveGridTextColorDark,
		)
	}

	if config.RecursiveGridTextColorLight != "#FF000000" {
		t.Errorf(
			"Expected RecursiveGridTextColorLight to be #FF000000, got %q",
			config.RecursiveGridTextColorLight,
		)
	}
}
