//go:build linux

//nolint:testpackage
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

func TestBuildStyle_UsesConfiguredMatchColors(t *testing.T) {
	cfg := config.DefaultConfig().Grid

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})

	if lightStyle.BackgroundColor != parseLinuxColor(config.GridBackgroundColorLight) {
		t.Fatalf("expected light background color to be resolved from config")
	}

	if lightStyle.MatchedBackgroundColor != parseLinuxColor(
		config.GridMatchedBackgroundColorLight,
	) {
		t.Fatalf("expected light matched background color to be resolved from config")
	}

	if lightStyle.MatchedBorderColor != parseLinuxColor(config.GridMatchedBorderColorLight) {
		t.Fatalf("expected light matched border color to be resolved from config")
	}

	if lightStyle.MatchedTextColor != parseLinuxColor(config.GridMatchedTextColorLight) {
		t.Fatalf("expected light matched text color to be resolved from config")
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})

	if darkStyle.BackgroundColor != parseLinuxColor(config.GridBackgroundColorDark) {
		t.Fatalf("expected dark background color to be resolved from config")
	}

	if darkStyle.MatchedBackgroundColor != parseLinuxColor(config.GridMatchedBackgroundColorDark) {
		t.Fatalf("expected dark matched background color to be resolved from config")
	}

	if darkStyle.MatchedBorderColor != parseLinuxColor(config.GridMatchedBorderColorDark) {
		t.Fatalf("expected dark matched border color to be resolved from config")
	}

	if darkStyle.MatchedTextColor != parseLinuxColor(config.GridMatchedTextColorDark) {
		t.Fatalf("expected dark matched text color to be resolved from config")
	}
}
