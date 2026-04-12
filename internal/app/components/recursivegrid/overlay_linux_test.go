//go:build linux

//nolint:testpackage
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

func TestBuildStyle_UsesLabelBackgroundAndPreviewConfig(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.LabelBackground = true
	cfg.UI.SubKeyPreview = true

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})

	if !lightStyle.LabelBackground {
		t.Fatal("expected label background to be enabled")
	}

	if !lightStyle.SubKeyPreview {
		t.Fatal("expected sub-key preview to be enabled")
	}

	if lightStyle.HighlightColor != parseLinuxColor(config.RecursiveGridHighlightColorLight) {
		t.Fatalf("expected light highlight color to be resolved from config")
	}

	if lightStyle.LabelBackgroundColor != parseLinuxColor(
		config.RecursiveGridLabelBackgroundColorLight,
	) {
		t.Fatalf("expected light label background color to be resolved from config")
	}

	if lightStyle.SubKeyPreviewTextColor != parseLinuxColor(
		config.RecursiveGridSubKeyPreviewTextColorLight,
	) {
		t.Fatalf("expected light sub-key preview color to be resolved from config")
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})

	if darkStyle.HighlightColor != parseLinuxColor(config.RecursiveGridHighlightColorDark) {
		t.Fatalf("expected dark highlight color to be resolved from config")
	}

	if darkStyle.LabelBackgroundColor != parseLinuxColor(
		config.RecursiveGridLabelBackgroundColorDark,
	) {
		t.Fatalf("expected dark label background color to be resolved from config")
	}

	if darkStyle.SubKeyPreviewTextColor != parseLinuxColor(
		config.RecursiveGridSubKeyPreviewTextColorDark,
	) {
		t.Fatalf("expected dark sub-key preview color to be resolved from config")
	}
}
