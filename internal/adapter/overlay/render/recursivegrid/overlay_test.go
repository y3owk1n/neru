//go:build darwin

package recursivegrid_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
)

type mockThemeProvider struct {
	darkMode bool
}

func (m *mockThemeProvider) IsDarkMode() bool {
	return m.darkMode
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
