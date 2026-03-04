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

func TestResolvedLabelColor_UserSpecified(t *testing.T) {
	// When user explicitly sets a label color, it should always be returned
	// regardless of the theme.
	userColor := "#FF0000FF" // Red

	darkTheme := &mockThemeProvider{darkMode: true}
	lightTheme := &mockThemeProvider{darkMode: false}

	result := config.ResolvedLabelColor(userColor, darkTheme)
	if result != userColor {
		t.Errorf("Expected user color %q in dark mode, got %q", userColor, result)
	}

	result = config.ResolvedLabelColor(userColor, lightTheme)
	if result != userColor {
		t.Errorf("Expected user color %q in light mode, got %q", userColor, result)
	}

	// Also with nil theme provider
	result = config.ResolvedLabelColor(userColor, nil)
	if result != userColor {
		t.Errorf("Expected user color %q with nil theme, got %q", userColor, result)
	}
}

func TestResolvedLabelColor_ThemeAwareDarkMode(t *testing.T) {
	// When label color is empty (theme-aware default) and Dark Mode is active,
	// should return white.
	darkTheme := &mockThemeProvider{darkMode: true}

	result := config.ResolvedLabelColor("", darkTheme)
	if result != config.LabelColorDarkMode {
		t.Errorf("Expected dark mode color %q, got %q", config.LabelColorDarkMode, result)
	}
}

func TestResolvedLabelColor_ThemeAwareLightMode(t *testing.T) {
	// When label color is empty (theme-aware default) and Light Mode is active,
	// should return black.
	lightTheme := &mockThemeProvider{darkMode: false}

	result := config.ResolvedLabelColor("", lightTheme)
	if result != config.LabelColorLightMode {
		t.Errorf("Expected light mode color %q, got %q", config.LabelColorLightMode, result)
	}
}

func TestResolvedLabelColor_NilThemeProvider(t *testing.T) {
	// When theme provider is nil and label color is empty,
	// should fall back to light mode color (safe default).
	result := config.ResolvedLabelColor("", nil)
	if result != config.LabelColorLightMode {
		t.Errorf("Expected light mode fallback %q, got %q", config.LabelColorLightMode, result)
	}
}

func TestDefaultConfig_LabelColorIsEmpty(t *testing.T) {
	// Verify that the default config uses empty string for LabelColor
	// (theme-aware sentinel).
	cfg := config.DefaultConfig()
	if cfg.RecursiveGrid.LabelColor != "" {
		t.Errorf("Expected empty LabelColor in default config, got %q", cfg.RecursiveGrid.LabelColor)
	}
}

func TestLabelColorConstants(t *testing.T) {
	// Verify the theme color constants are valid hex colors.
	if config.LabelColorDarkMode != "#FFFFFFFF" {
		t.Errorf("Expected LabelColorDarkMode to be #FFFFFFFF, got %q", config.LabelColorDarkMode)
	}

	if config.LabelColorLightMode != "#FF000000" {
		t.Errorf("Expected LabelColorLightMode to be #FF000000, got %q", config.LabelColorLightMode)
	}
}

