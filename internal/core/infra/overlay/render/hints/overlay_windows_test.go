//go:build windows

package hints //nolint:testpackage

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

func TestBuildStyle_UsesThemeAwareConfigValues(t *testing.T) {
	cfg := config.DefaultConfig().Hints

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})

	if lightStyle.BackgroundColor() != config.HintsBackgroundColorLight {
		t.Fatalf(
			"expected light background %q, got %q",
			config.HintsBackgroundColorLight,
			lightStyle.BackgroundColor(),
		)
	}

	if lightStyle.TextColor() != config.HintsTextColorLight {
		t.Fatalf(
			"expected light text %q, got %q",
			config.HintsTextColorLight,
			lightStyle.TextColor(),
		)
	}

	if lightStyle.BorderColor() != config.HintsBorderColorLight {
		t.Fatalf(
			"expected light border %q, got %q",
			config.HintsBorderColorLight,
			lightStyle.BorderColor(),
		)
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})

	if darkStyle.BackgroundColor() != config.HintsBackgroundColorDark {
		t.Fatalf(
			"expected dark background %q, got %q",
			config.HintsBackgroundColorDark,
			darkStyle.BackgroundColor(),
		)
	}

	if darkStyle.TextColor() != config.HintsTextColorDark {
		t.Fatalf(
			"expected dark text %q, got %q",
			config.HintsTextColorDark,
			darkStyle.TextColor(),
		)
	}
}
