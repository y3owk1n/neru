//go:build darwin

package recursivegrid_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
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
	cfg.UI.LabelBackground = true

	lightStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: false})
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
		t.Fatalf(
			"expected light text color %q, got %q",
			config.RecursiveGridTextColorLight,
			lightStyle.TextColor(),
		)
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

	if lightStyle.LabelBackgroundBorderRadius() != config.DefaultRecursiveGridLabelBackgroundBorderRadius {
		t.Fatalf(
			"expected default label background border radius %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundBorderRadius,
			lightStyle.LabelBackgroundBorderRadius(),
		)
	}

	if lightStyle.LabelBackgroundBorderWidth() != config.DefaultRecursiveGridLabelBackgroundBorderWidth {
		t.Fatalf(
			"expected default label background border width %d, got %d",
			config.DefaultRecursiveGridLabelBackgroundBorderWidth,
			lightStyle.LabelBackgroundBorderWidth(),
		)
	}

	darkStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackgroundColor() != config.RecursiveGridLabelBackgroundColorDark {
		t.Fatalf(
			"expected dark label background color %q, got %q",
			config.RecursiveGridLabelBackgroundColorDark,
			darkStyle.LabelBackgroundColor(),
		)
	}

	if darkStyle.TextColor() != config.RecursiveGridTextColorDark {
		t.Fatalf(
			"expected dark text color %q, got %q",
			config.RecursiveGridTextColorDark,
			darkStyle.TextColor(),
		)
	}
}

func TestBuildStyle_UsesUserSpecifiedLabelBackgroundColors(t *testing.T) {
	const (
		customLightLabelBG = "#11223344"
		customDarkLabelBG  = "#55667788"
	)

	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.LabelBackground = true
	cfg.UI.LabelBackgroundColor = config.Color{Light: customLightLabelBG, Dark: customDarkLabelBG}
	cfg.UI.TextColor = config.Color{Light: "#FF111111", Dark: "#FFEEEEEE"}
	cfg.UI.LabelBackgroundPaddingX = 9
	cfg.UI.LabelBackgroundPaddingY = 5
	cfg.UI.LabelBackgroundBorderRadius = 3
	cfg.UI.LabelBackgroundBorderWidth = 2

	lightStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.LabelBackgroundColor() != customLightLabelBG {
		t.Fatalf(
			"expected custom light label background color, got %q",
			lightStyle.LabelBackgroundColor(),
		)
	}

	if lightStyle.TextColor() != "#FF111111" {
		t.Fatalf("expected custom light text color, got %q", lightStyle.TextColor())
	}

	if lightStyle.LabelBackgroundPaddingX() != 9 {
		t.Fatalf(
			"expected custom light label background padding x, got %d",
			lightStyle.LabelBackgroundPaddingX(),
		)
	}

	if lightStyle.LabelBackgroundPaddingY() != 5 {
		t.Fatalf(
			"expected custom light label background padding y, got %d",
			lightStyle.LabelBackgroundPaddingY(),
		)
	}

	if lightStyle.LabelBackgroundBorderRadius() != 3 {
		t.Fatalf(
			"expected custom light label background border radius, got %d",
			lightStyle.LabelBackgroundBorderRadius(),
		)
	}

	if lightStyle.LabelBackgroundBorderWidth() != 2 {
		t.Fatalf(
			"expected custom light label background border width, got %d",
			lightStyle.LabelBackgroundBorderWidth(),
		)
	}

	darkStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackgroundColor() != customDarkLabelBG {
		t.Fatalf(
			"expected custom dark label background color, got %q",
			darkStyle.LabelBackgroundColor(),
		)
	}

	if darkStyle.TextColor() != "#FFEEEEEE" {
		t.Fatalf("expected custom dark text color, got %q", darkStyle.TextColor())
	}

	if darkStyle.LabelBackgroundPaddingX() != 9 {
		t.Fatalf(
			"expected custom dark label background padding x, got %d",
			darkStyle.LabelBackgroundPaddingX(),
		)
	}

	if darkStyle.LabelBackgroundPaddingY() != 5 {
		t.Fatalf(
			"expected custom dark label background padding y, got %d",
			darkStyle.LabelBackgroundPaddingY(),
		)
	}

	if darkStyle.LabelBackgroundBorderRadius() != 3 {
		t.Fatalf(
			"expected custom dark label background border radius, got %d",
			darkStyle.LabelBackgroundBorderRadius(),
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
	cfg.UI.LabelBackground = false
	cfg.UI.HighlightColor = config.Color{Light: "#11442266", Dark: "#228844AA"}
	cfg.UI.TextColor = config.Color{Light: "#FF101010", Dark: "#FFF0F0F0"}
	cfg.UI.LabelBackgroundColor = config.Color{Light: "#CCFFD700", Dark: "#99FFD700"}

	lightStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.LabelBackground() {
		t.Fatal("expected label background to be disabled")
	}

	if lightStyle.HighlightColor() != "#11442266" {
		t.Fatalf(
			"expected light highlight color to remain unchanged, got %q",
			lightStyle.HighlightColor(),
		)
	}

	if lightStyle.TextColor() != "#FF101010" {
		t.Fatalf("expected light text color to remain unchanged, got %q", lightStyle.TextColor())
	}

	darkStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.LabelBackground() {
		t.Fatal("expected label background to be disabled")
	}

	if darkStyle.HighlightColor() != "#228844AA" {
		t.Fatalf(
			"expected dark highlight color to remain unchanged, got %q",
			darkStyle.HighlightColor(),
		)
	}

	if darkStyle.TextColor() != "#FFF0F0F0" {
		t.Fatalf("expected dark text color to remain unchanged, got %q", darkStyle.TextColor())
	}
}

func TestBuildStyle_LabelBackgroundEnabledUsesDedicatedBadgeColor(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid
	cfg.UI.LabelBackground = true
	cfg.UI.HighlightColor = config.Color{Light: "#11223344", Dark: "#55667788"}
	cfg.UI.LabelBackgroundColor = config.Color{Light: "#99ABCDEF", Dark: "#66FEDCBA"}

	lightStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.HighlightColor() != "#11223344" {
		t.Fatalf(
			"expected light highlight color to remain unchanged, got %q",
			lightStyle.HighlightColor(),
		)
	}

	if lightStyle.LabelBackgroundColor() != "#99ABCDEF" {
		t.Fatalf(
			"expected light badge color to use dedicated config, got %q",
			lightStyle.LabelBackgroundColor(),
		)
	}

	darkStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.HighlightColor() != "#55667788" {
		t.Fatalf(
			"expected dark highlight color to remain unchanged, got %q",
			darkStyle.HighlightColor(),
		)
	}

	if darkStyle.LabelBackgroundColor() != "#66FEDCBA" {
		t.Fatalf(
			"expected dark badge color to use dedicated config, got %q",
			darkStyle.LabelBackgroundColor(),
		)
	}
}

func TestBuildStyle_SubKeyPreviewUsesMainLabelColorByDefault(t *testing.T) {
	cfg := config.DefaultConfig().RecursiveGrid

	lightStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: false})
	if lightStyle.SubKeyPreviewTextColor() != lightStyle.TextColor() {
		t.Fatalf(
			"expected light subkey preview color %q to match main text color %q",
			lightStyle.SubKeyPreviewTextColor(),
			lightStyle.TextColor(),
		)
	}

	darkStyle := recursivegrid.BuildStyle(cfg, &mockThemeProvider{darkMode: true})
	if darkStyle.SubKeyPreviewTextColor() != darkStyle.TextColor() {
		t.Fatalf(
			"expected dark subkey preview color %q to match main text color %q",
			darkStyle.SubKeyPreviewTextColor(),
			darkStyle.TextColor(),
		)
	}
}
