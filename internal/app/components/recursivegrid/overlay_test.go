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

func TestBuildStyle_UsesDefaultLabelBackgroundColors(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.LabelBackground = true

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if !lightStyle.LabelBackground() {
		t.Fatal("expected label background to be enabled")
	}
	if lightStyle.LabelBackgroundColor() != config.RecursiveGridLabelBackgroundColorLight {
		t.Fatalf(
			"expected light label background color %q, got %q",
			config.RecursiveGridLabelBackgroundColorLight,
			lightStyle.LabelBackgroundColor(),
		)
	}
	if lightStyle.TextColor() != config.RecursiveGridTextColorLight {
		t.Fatalf("expected light text color %q, got %q", config.RecursiveGridTextColorLight, lightStyle.TextColor())
	}
	if lightStyle.LabelBackgroundPaddingX() != config.DefaultRecursiveGridLabelBackgroundPaddingX {
		t.Fatalf(
			"expected default label background padding x %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundPaddingX,
			lightStyle.LabelBackgroundPaddingX(),
		)
	}
	if lightStyle.LabelBackgroundPaddingY() != config.DefaultRecursiveGridLabelBackgroundPaddingY {
		t.Fatalf(
			"expected default label background padding y %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundPaddingY,
			lightStyle.LabelBackgroundPaddingY(),
		)
	}
	if lightStyle.LabelBackgroundCornerRadius() != config.DefaultRecursiveGridLabelBackgroundCornerRadius {
		t.Fatalf(
			"expected default label background corner radius %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundCornerRadius,
			lightStyle.LabelBackgroundCornerRadius(),
		)
	}
	if lightStyle.LabelBackgroundBorderWidth() != config.DefaultRecursiveGridLabelBackgroundBorderWidth {
		t.Fatalf(
			"expected default label background border width %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundBorderWidth,
			lightStyle.LabelBackgroundBorderWidth(),
		)
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackgroundColor() != config.RecursiveGridLabelBackgroundColorDark {
		t.Fatalf(
			"expected dark label background color %q, got %q",
			config.RecursiveGridLabelBackgroundColorDark,
			darkStyle.LabelBackgroundColor(),
		)
	}
	if darkStyle.TextColor() != config.RecursiveGridTextColorDark {
		t.Fatalf("expected dark text color %q, got %q", config.RecursiveGridTextColorDark, darkStyle.TextColor())
	}
}

func TestBuildStyle_UsesUserSpecifiedLabelBackgroundColors(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.LabelBackground = true
	cfg.LabelBackgroundColorLight = "#11223344"
	cfg.LabelBackgroundColorDark = "#55667788"
	cfg.TextColorLight = "#FF111111"
	cfg.TextColorDark = "#FFEEEEEE"
	cfg.LabelBackgroundPaddingX = 9
	cfg.LabelBackgroundPaddingY = 5
	cfg.LabelBackgroundCornerRadius = 3
	cfg.LabelBackgroundBorderWidth = 2

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.LabelBackgroundColor() != "#11223344" {
		t.Fatalf("expected custom light label background color, got %q", lightStyle.LabelBackgroundColor())
	}
	if lightStyle.TextColor() != "#FF111111" {
		t.Fatalf("expected custom light text color, got %q", lightStyle.TextColor())
	}
	if lightStyle.LabelBackgroundPaddingX() != 9 {
		t.Fatalf("expected custom light label background padding x, got %d", lightStyle.LabelBackgroundPaddingX())
	}
	if lightStyle.LabelBackgroundPaddingY() != 5 {
		t.Fatalf("expected custom light label background padding y, got %d", lightStyle.LabelBackgroundPaddingY())
	}
	if lightStyle.LabelBackgroundCornerRadius() != 3 {
		t.Fatalf(
			"expected custom light label background corner radius, got %d",
			lightStyle.LabelBackgroundCornerRadius(),
		)
	}
	if lightStyle.LabelBackgroundBorderWidth() != 2 {
		t.Fatalf(
			"expected custom light label background border width, got %d",
			lightStyle.LabelBackgroundBorderWidth(),
		)
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackgroundColor() != "#55667788" {
		t.Fatalf("expected custom dark label background color, got %q", darkStyle.LabelBackgroundColor())
	}
	if darkStyle.TextColor() != "#FFEEEEEE" {
		t.Fatalf("expected custom dark text color, got %q", darkStyle.TextColor())
	}
	if darkStyle.LabelBackgroundPaddingX() != 9 {
		t.Fatalf("expected custom dark label background padding x, got %d", darkStyle.LabelBackgroundPaddingX())
	}
	if darkStyle.LabelBackgroundPaddingY() != 5 {
		t.Fatalf("expected custom dark label background padding y, got %d", darkStyle.LabelBackgroundPaddingY())
	}
	if darkStyle.LabelBackgroundCornerRadius() != 3 {
		t.Fatalf(
			"expected custom dark label background corner radius, got %d",
			darkStyle.LabelBackgroundCornerRadius(),
		)
	}
	if darkStyle.LabelBackgroundBorderWidth() != 2 {
		t.Fatalf(
			"expected custom dark label background border width, got %d",
			darkStyle.LabelBackgroundBorderWidth(),
		)
	}
}

func TestBuildStyle_LabelBackgroundDisabledPreservesNormalGridColors(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.LabelBackground = false
	cfg.HighlightColorLight = "#11442266"
	cfg.HighlightColorDark = "#228844AA"
	cfg.TextColorLight = "#FF101010"
	cfg.TextColorDark = "#FFF0F0F0"
	cfg.LabelBackgroundColorLight = "#CCFFD700"
	cfg.LabelBackgroundColorDark = "#99FFD700"

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.LabelBackground() {
		t.Fatal("expected label background to be disabled")
	}
	if lightStyle.HighlightColor() != "#11442266" {
		t.Fatalf("expected light highlight color to remain unchanged, got %q", lightStyle.HighlightColor())
	}
	if lightStyle.TextColor() != "#FF101010" {
		t.Fatalf("expected light text color to remain unchanged, got %q", lightStyle.TextColor())
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackground() {
		t.Fatal("expected label background to be disabled")
	}
	if darkStyle.HighlightColor() != "#228844AA" {
		t.Fatalf("expected dark highlight color to remain unchanged, got %q", darkStyle.HighlightColor())
	}
	if darkStyle.TextColor() != "#FFF0F0F0" {
		t.Fatalf("expected dark text color to remain unchanged, got %q", darkStyle.TextColor())
	}
}

func TestBuildStyle_LabelBackgroundEnabledUsesDedicatedBadgeColor(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.LabelBackground = true
	cfg.HighlightColorLight = "#11223344"
	cfg.HighlightColorDark = "#55667788"
	cfg.LabelBackgroundColorLight = "#99ABCDEF"
	cfg.LabelBackgroundColorDark = "#66FEDCBA"

	lightStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.HighlightColor() != "#11223344" {
		t.Fatalf("expected light highlight color to remain unchanged, got %q", lightStyle.HighlightColor())
	}
	if lightStyle.LabelBackgroundColor() != "#99ABCDEF" {
		t.Fatalf("expected light badge color to use dedicated config, got %q", lightStyle.LabelBackgroundColor())
	}

	darkStyle := BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.HighlightColor() != "#55667788" {
		t.Fatalf("expected dark highlight color to remain unchanged, got %q", darkStyle.HighlightColor())
	}
	if darkStyle.LabelBackgroundColor() != "#66FEDCBA" {
		t.Fatalf("expected dark badge color to use dedicated config, got %q", darkStyle.LabelBackgroundColor())
	}
}
