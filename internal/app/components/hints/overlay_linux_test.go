//go:build linux

//nolint:testpackage
package hints

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

	if lightStyle.FontSize() != cfg.UI.FontSize {
		t.Fatalf("expected font size %d, got %d", cfg.UI.FontSize, lightStyle.FontSize())
	}

	if lightStyle.FontFamily() != cfg.UI.FontFamily {
		t.Fatalf("expected font family %q, got %q", cfg.UI.FontFamily, lightStyle.FontFamily())
	}

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

	if lightStyle.MatchedTextColor() != config.HintsMatchedTextColorLight {
		t.Fatalf(
			"expected light matched text %q, got %q",
			config.HintsMatchedTextColorLight,
			lightStyle.MatchedTextColor(),
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
		t.Fatalf("expected dark text %q, got %q", config.HintsTextColorDark, darkStyle.TextColor())
	}

	if darkStyle.MatchedTextColor() != config.HintsMatchedTextColorDark {
		t.Fatalf(
			"expected dark matched text %q, got %q",
			config.HintsMatchedTextColorDark,
			darkStyle.MatchedTextColor(),
		)
	}

	if darkStyle.BorderColor() != config.HintsBorderColorDark {
		t.Fatalf(
			"expected dark border %q, got %q",
			config.HintsBorderColorDark,
			darkStyle.BorderColor(),
		)
	}
}
