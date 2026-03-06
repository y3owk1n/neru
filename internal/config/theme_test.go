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

const (
	testDefaultLight = "#AAAAAA"
	testDefaultDark  = "#BBBBBB"
)

func TestResolveColor_UserSpecified(t *testing.T) {
	// When user explicitly sets colors, they should be used.
	lightColor := "#111111"
	darkColor := "#EEEEEE"
	defaultLight := testDefaultLight
	defaultDark := testDefaultDark

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

func TestResolveColor_OnlyLightSet(t *testing.T) {
	// When only light color is set, dark mode should fall back to defaultDark.
	lightColor := "#111111"
	defaultLight := testDefaultLight
	defaultDark := testDefaultDark
	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}
	// Light mode uses the user-specified light color
	result := config.ResolveColor(lightColor, "", lightTheme, defaultLight, defaultDark)
	if result != lightColor {
		t.Errorf("Expected light color %q, got %q", lightColor, result)
	}
	// Dark mode falls back to default dark (not the light color)
	result = config.ResolveColor(lightColor, "", darkTheme, defaultLight, defaultDark)
	if result != defaultDark {
		t.Errorf("Expected default dark %q, got %q", defaultDark, result)
	}
	// Nil theme uses the user-specified light color
	result = config.ResolveColor(lightColor, "", nil, defaultLight, defaultDark)
	if result != lightColor {
		t.Errorf("Expected light color %q with nil theme, got %q", lightColor, result)
	}
}

func TestResolveColor_OnlyDarkSet(t *testing.T) {
	// When only dark color is set, light mode should fall back to defaultLight.
	darkColor := "#EEEEEE"
	defaultLight := testDefaultLight
	defaultDark := testDefaultDark
	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}
	// Dark mode uses the user-specified dark color
	result := config.ResolveColor("", darkColor, darkTheme, defaultLight, defaultDark)
	if result != darkColor {
		t.Errorf("Expected dark color %q, got %q", darkColor, result)
	}
	// Light mode falls back to default light (not the dark color)
	result = config.ResolveColor("", darkColor, lightTheme, defaultLight, defaultDark)
	if result != defaultLight {
		t.Errorf("Expected default light %q, got %q", defaultLight, result)
	}
	// Nil theme falls back to default light
	result = config.ResolveColor("", darkColor, nil, defaultLight, defaultDark)
	if result != defaultLight {
		t.Errorf("Expected default light %q with nil theme, got %q", defaultLight, result)
	}
}

func TestResolveColor_ThemeAwareDefaults(t *testing.T) {
	// When both light and dark colors are empty, use provided defaults.
	defaultLight := testDefaultLight
	defaultDark := testDefaultDark

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
