package loader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// testSurface is a theme surface no default palette uses, so a derived color
// carrying it can only have come from the layer under test.
const (
	testSurface           = "#123456"
	testSurfaceBackground = "#F2123456"
)

// themeConfig is a config file that sets nothing but the light surface, so the
// component colors a load produces are the derivation under test.
func themeConfig(surface string) string {
	return "[theme.light]\nsurface = \"" + surface + "\"\n"
}

// loadWithOverride writes a config file and its override file, loads them, and
// hands back the result. Both files are the whole configuration each layer
// contributes; neither is merged by the test.
func loadWithOverride(t *testing.T, base, override string) *config.LoadResult {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	err := os.WriteFile(configPath, []byte(base), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if override != "" {
		overridePath := loader.OverridePath(configPath)

		err := os.WriteFile(overridePath, []byte(override), 0o600)
		if err != nil {
			t.Fatalf("failed to write override: %v", err)
		}
	}

	service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(configPath)
	if result.ValidationError != nil {
		t.Fatalf("load failed: %v", result.ValidationError)
	}

	return result
}

// TestLoadResolvesThemeAfterOverrides is the theme half of the ordering
// TestLoadResolvesGridLabelsAfterOverrides pins for grid labels. Component
// colors resolved before the override layer are resolved from a palette the
// daemon does not run on — and because every component color comes out of that
// first pass non-empty, the second pass has nothing left to fill, so a theme in
// the override file used to reach nothing at all.
func TestLoadResolvesThemeAfterOverrides(t *testing.T) {
	result := loadWithOverride(t, "[general]\nlog_level = \"info\"\n", themeConfig(testSurface))

	if got := result.Config.Theme.Light.Surface; got != testSurface {
		t.Fatalf("Theme.Light.Surface = %q, want %q", got, testSurface)
	}

	if got := result.Config.Hints.UI.BackgroundColor.Light; got != testSurfaceBackground {
		t.Errorf(
			"Hints.UI.BackgroundColor.Light = %q, want %q (derived from the override's surface)",
			got, testSurfaceBackground,
		)
	}
}

// TestLoadReportsWhatTheUserWrote pins the second half of a load: beside the
// configuration the daemon runs on is the one it was derived from, which is
// what lets a later field change derive again instead of re-deriving from its
// own output.
func TestLoadReportsWhatTheUserWrote(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	if result.Written == nil {
		t.Fatal("LoadResult.Written is nil")
	}

	if got := result.Written.Grid.RowLabels; got != "" {
		t.Errorf("Written.Grid.RowLabels = %q, want %q (nobody wrote labels)", got, "")
	}

	if got := result.Written.Hints.UI.BackgroundColor.Light; got != "" {
		t.Errorf(
			"Written.Hints.UI.BackgroundColor.Light = %q, want %q (nobody wrote a color)",
			got, "",
		)
	}

	if got := result.Config.Grid.RowLabels; got != testGridLabels {
		t.Errorf("Config.Grid.RowLabels = %q, want %q", got, testGridLabels)
	}
}

// TestApplyFieldChangeReinfersGridLabels is the bug this pair exists for.
// `neru config set` applies in memory rather than re-reading the file, so
// before the written configuration was kept beside the resolved one, a change
// to grid.characters landed on labels that were already settled — and settled
// labels are indistinguishable from labels a user typed, so the grid kept
// drawing the alphabet inferred from the characters it no longer uses.
func TestApplyFieldChangeReinfersGridLabels(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	running, written, err := loader.ApplyFieldChange(
		result.Written,
		"grid.characters",
		testGridCharsAlt,
	)
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if got := running.Grid.RowLabels; got != testGridLabelsAlt {
		t.Errorf("Grid.RowLabels = %q, want %q", got, testGridLabelsAlt)
	}

	if got := running.Grid.ColLabels; got != testGridLabelsAlt {
		t.Errorf("Grid.ColLabels = %q, want %q", got, testGridLabelsAlt)
	}

	if got := written.Grid.RowLabels; got != "" {
		t.Errorf("Written.Grid.RowLabels = %q, want %q (still nobody wrote labels)", got, "")
	}
}

// TestApplyFieldChangeReinfersThemeColors is the same class one derived value
// over: a theme change has to reach the component colors derived from it.
func TestApplyFieldChangeReinfersThemeColors(t *testing.T) {
	result := loadWithOverride(t, "[general]\nlog_level = \"info\"\n", "")

	running, _, err := loader.ApplyFieldChange(result.Written, "theme.light.surface", testSurface)
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if got := running.Hints.UI.BackgroundColor.Light; got != testSurfaceBackground {
		t.Errorf(
			"Hints.UI.BackgroundColor.Light = %q, want %q",
			got, testSurfaceBackground,
		)
	}
}

// TestApplyFieldChangeNormalizesADerivedField guards the other direction:
// setting the derived option itself still records what was typed and settles
// it, rather than being overwritten by an inference.
func TestApplyFieldChangeNormalizesADerivedField(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	running, written, err := loader.ApplyFieldChange(result.Written, "grid.row_labels", "xy")
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if got := running.Grid.RowLabels; got != "XY" {
		t.Errorf("Grid.RowLabels = %q, want %q", got, "XY")
	}

	if got := running.Grid.ColLabels; got != testGridLabels {
		t.Errorf("Grid.ColLabels = %q, want %q", got, testGridLabels)
	}

	if got := written.Grid.RowLabels; got != "xy" {
		t.Errorf("Written.Grid.RowLabels = %q, want %q (what was typed)", got, "xy")
	}
}

// TestApplyFieldChangeLeavesTheSourceAlone pins that the pair handed in is not
// the pair handed back. `neru config set` can fail validation after the change
// is applied, and the configuration the daemon is running on has to survive
// that unchanged.
func TestApplyFieldChangeLeavesTheSourceAlone(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	_, _, err := loader.ApplyFieldChange(result.Written, "grid.characters", testGridCharsAlt)
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if got := result.Written.Grid.Characters; got != testGridChars {
		t.Errorf("source Written.Grid.Characters = %q, want %q", got, testGridChars)
	}
}

// TestApplyFieldChangeRejectsAnUnknownField keeps the failure at the field
// change rather than at the resolution behind it.
func TestApplyFieldChangeRejectsAnUnknownField(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	_, _, err := loader.ApplyFieldChange(result.Written, "grid.no_such_option", "1")
	if err == nil {
		t.Fatal("ApplyFieldChange accepted an unknown field")
	}
}

// TestLoadValidatesTheLayeredConfiguration pins which configuration a refusal
// is a judgement of. The layers used to be validated one at a time, so a config
// file that was invalid on its own dropped the daemon to the defaults even when
// the override file answered for it — a reading of a configuration that was
// never going to run. There is one reading now, of the one that will.
func TestLoadValidatesTheLayeredConfiguration(t *testing.T) {
	// grid.characters must not be blank while grid is enabled; the override
	// answers for it.
	result := loadWithOverride(
		t,
		"[grid]\nenabled = true\ncharacters = \"\"\n",
		gridConfigWithCharacters(testGridChars),
	)

	if got := result.Config.Grid.Characters; got != testGridChars {
		t.Errorf("Grid.Characters = %q, want %q", got, testGridChars)
	}

	if got := result.Config.Grid.RowLabels; got != testGridLabels {
		t.Errorf("Grid.RowLabels = %q, want %q", got, testGridLabels)
	}
}

// TestApplyFieldChangeDropsLaunchersForADisabledMode covers the derivation that
// is the loader's rather than the schema's. Switching a mode off at runtime has
// to take its launcher binding with it, the way a reload does — and switching
// it back on has to bring the binding back, which only works because the
// written configuration still has it.
func TestApplyFieldChangeDropsLaunchersForADisabledMode(t *testing.T) {
	result := loadWithOverride(t, gridConfigWithCharacters(testGridChars), "")

	if !hasLauncherFor(result.Config, config.ModeNameGrid) {
		t.Fatal("the loaded configuration has no grid launcher binding to drop")
	}

	off, offWritten, err := loader.ApplyFieldChange(result.Written, "grid.enabled", "false")
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if hasLauncherFor(off, config.ModeNameGrid) {
		t.Error("disabling grid left its launcher binding in the running configuration")
	}

	on, _, err := loader.ApplyFieldChange(offWritten, "grid.enabled", "true")
	if err != nil {
		t.Fatalf("ApplyFieldChange failed: %v", err)
	}

	if !hasLauncherFor(on, config.ModeNameGrid) {
		t.Error("re-enabling grid did not bring its launcher binding back")
	}
}

// hasLauncherFor reports whether any global binding launches the named mode.
func hasLauncherFor(cfg *config.Config, mode string) bool {
	for _, actions := range cfg.Hotkeys.Bindings {
		if len(actions) == 1 && strings.Fields(actions[0])[0] == mode {
			return true
		}
	}

	return false
}
