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
	testDefaultLight  = "#AAAAAA"
	testDefaultDark   = "#BBBBBB"
	testUILight       = "#330000"
	testUIDark        = "#440000"
	testUserLight     = "#111111"
	testUserDark      = "#EEEEEE"
	testOverrideLight = "#110000"
	testOverrideDark  = "#220000"
)

func TestForTheme_UserSpecified(t *testing.T) {
	// When user explicitly sets colors, they should be used.
	_config := config.Color{Light: testUserLight, Dark: testUserDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks dark color
	result := _config.ForTheme(darkTheme, testDefaultLight, testDefaultDark)
	if result != testUserDark {
		t.Errorf("Expected dark color %q, got %q", testUserDark, result)
	}

	// Light mode picks light color
	result = _config.ForTheme(lightTheme, testDefaultLight, testDefaultDark)
	if result != testUserLight {
		t.Errorf("Expected light color %q, got %q", testUserLight, result)
	}

	// Nil theme picks light color
	result = _config.ForTheme(nil, testDefaultLight, testDefaultDark)
	if result != testUserLight {
		t.Errorf("Expected light color fallback %q, got %q", testUserLight, result)
	}
}

func TestForTheme_OnlyLightSet(t *testing.T) {
	// When only light color is set, dark mode should fall back to defaultDark.
	_config := config.Color{Light: testUserLight}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Light mode uses the user-specified light color
	result := _config.ForTheme(lightTheme, testDefaultLight, testDefaultDark)
	if result != testUserLight {
		t.Errorf("Expected light color %q, got %q", testUserLight, result)
	}

	// Dark mode falls back to default dark (not the light color)
	result = _config.ForTheme(darkTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultDark {
		t.Errorf("Expected default dark %q, got %q", testDefaultDark, result)
	}

	// Nil theme uses the user-specified light color
	result = _config.ForTheme(nil, testDefaultLight, testDefaultDark)
	if result != testUserLight {
		t.Errorf("Expected light color %q with nil theme, got %q", testUserLight, result)
	}
}

func TestForTheme_OnlyDarkSet(t *testing.T) {
	// When only dark color is set, light mode should fall back to defaultLight.
	_config := config.Color{Dark: testUserDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode uses the user-specified dark color
	result := _config.ForTheme(darkTheme, testDefaultLight, testDefaultDark)
	if result != testUserDark {
		t.Errorf("Expected dark color %q, got %q", testUserDark, result)
	}

	// Light mode falls back to default light (not the dark color)
	result = _config.ForTheme(lightTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultLight {
		t.Errorf("Expected default light %q, got %q", testDefaultLight, result)
	}

	// Nil theme falls back to default light
	result = _config.ForTheme(nil, testDefaultLight, testDefaultDark)
	if result != testDefaultLight {
		t.Errorf("Expected default light %q with nil theme, got %q", testDefaultLight, result)
	}
}

func TestForThemeWithOverride_BothOverridesSet(t *testing.T) {
	// When both per-mode overrides are set, they take precedence over
	// shared UI defaults and hardcoded defaults.
	_config := config.Color{Light: testOverrideLight, Dark: testOverrideDark}
	uiDefault := config.Color{Light: testUILight, Dark: testUIDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks per-mode dark override
	result := _config.ForThemeWithOverride(uiDefault, darkTheme, testDefaultLight, testDefaultDark)
	if result != testOverrideDark {
		t.Errorf("Expected per-mode dark override %q, got %q", testOverrideDark, result)
	}

	// Light mode picks per-mode light override
	result = _config.ForThemeWithOverride(uiDefault, lightTheme, testDefaultLight, testDefaultDark)
	if result != testOverrideLight {
		t.Errorf("Expected per-mode light override %q, got %q", testOverrideLight, result)
	}
}

func TestForThemeWithOverride_OnlyLightOverrideSet(t *testing.T) {
	// When only the light per-mode override is set, dark mode should
	// fall back to the shared UI dark value (not the hardcoded default).
	_config := config.Color{Light: testOverrideLight}
	uiDefault := config.Color{Light: testUILight, Dark: testUIDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Light mode picks per-mode light override
	result := _config.ForThemeWithOverride(uiDefault, lightTheme, testDefaultLight, testDefaultDark)
	if result != testOverrideLight {
		t.Errorf("Expected per-mode light override %q, got %q", testOverrideLight, result)
	}

	// Dark mode falls back to shared UI dark (middle tier), not hardcoded default
	result = _config.ForThemeWithOverride(uiDefault, darkTheme, testDefaultLight, testDefaultDark)
	if result != testUIDark {
		t.Errorf("Expected shared UI dark %q, got %q", testUIDark, result)
	}
}

func TestForThemeWithOverride_OnlyDarkOverrideSet(t *testing.T) {
	// When only the dark per-mode override is set, light mode should
	// fall back to the shared UI light value (not the hardcoded default).
	_config := config.Color{Dark: testOverrideDark}
	uiDefault := config.Color{Light: testUILight, Dark: testUIDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks per-mode dark override
	result := _config.ForThemeWithOverride(uiDefault, darkTheme, testDefaultLight, testDefaultDark)
	if result != testOverrideDark {
		t.Errorf("Expected per-mode dark override %q, got %q", testOverrideDark, result)
	}

	// Light mode falls back to shared UI light (middle tier), not hardcoded default
	result = _config.ForThemeWithOverride(uiDefault, lightTheme, testDefaultLight, testDefaultDark)
	if result != testUILight {
		t.Errorf("Expected shared UI light %q, got %q", testUILight, result)
	}
}

func TestForThemeWithOverride_NoOverrides(t *testing.T) {
	// When no per-mode overrides are set, falls through to shared UI defaults.
	_config := config.Color{}
	uiDefault := config.Color{Light: testUILight, Dark: testUIDark}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks shared UI dark
	result := _config.ForThemeWithOverride(uiDefault, darkTheme, testDefaultLight, testDefaultDark)
	if result != testUIDark {
		t.Errorf("Expected shared UI dark %q, got %q", testUIDark, result)
	}

	// Light mode picks shared UI light
	result = _config.ForThemeWithOverride(uiDefault, lightTheme, testDefaultLight, testDefaultDark)
	if result != testUILight {
		t.Errorf("Expected shared UI light %q, got %q", testUILight, result)
	}
}

func TestForThemeWithOverride_NoOverridesNoUI(t *testing.T) {
	// When no overrides and no shared UI values are set, falls through
	// to hardcoded defaults.
	_config := config.Color{}
	uiDefault := config.Color{}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks hardcoded default dark
	result := _config.ForThemeWithOverride(uiDefault, darkTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultDark {
		t.Errorf("Expected hardcoded default dark %q, got %q", testDefaultDark, result)
	}

	// Light mode picks hardcoded default light
	result = _config.ForThemeWithOverride(uiDefault, lightTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultLight {
		t.Errorf("Expected hardcoded default light %q, got %q", testDefaultLight, result)
	}
}

func TestForTheme_ThemeAwareDefaults(t *testing.T) {
	// When both light and dark colors are empty, use provided defaults.
	_config := config.Color{}

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	// Dark mode picks default dark
	result := _config.ForTheme(darkTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultDark {
		t.Errorf("Expected default dark %q, got %q", testDefaultDark, result)
	}

	// Light mode picks default light
	result = _config.ForTheme(lightTheme, testDefaultLight, testDefaultDark)
	if result != testDefaultLight {
		t.Errorf("Expected default light %q, got %q", testDefaultLight, result)
	}
}
