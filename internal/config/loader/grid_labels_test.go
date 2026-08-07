package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// testGridChars is the character set the fixtures configure, and
// testGridLabels the labels a grid built from it carries.
const (
	testGridChars  = "asdf"
	testGridLabels = "ASDF"
	// The second set, for the tests that change the characters and expect the
	// labels to follow.
	testGridCharsAlt  = "qwer"
	testGridLabelsAlt = "QWER"
)

// gridConfigWithCharacters is a config file that sets nothing but the grid
// coordinate characters, so the labels a load produces are the derivation
// under test and not something the file said.
func gridConfigWithCharacters(characters string) string {
	return "[grid]\nenabled = true\ncharacters = \"" + characters + "\"\n"
}

// TestLoadResolvesGridLabels pins that a loaded config answers "what labels is
// the grid using" itself. The option means "infer from characters" when it is
// empty, and until this resolution existed that meaning was implemented in a
// consumer rather than in the config the consumer was handed.
func TestLoadResolvesGridLabels(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	writeErr := os.WriteFile(configPath, []byte(gridConfigWithCharacters(testGridChars)), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(configPath)
	if result.ValidationError != nil {
		t.Fatalf("load failed: %v", result.ValidationError)
	}

	if result.Config.Grid.RowLabels != testGridLabels {
		t.Errorf("Grid.RowLabels = %q, want %q", result.Config.Grid.RowLabels, testGridLabels)
	}

	if result.Config.Grid.ColLabels != testGridLabels {
		t.Errorf("Grid.ColLabels = %q, want %q", result.Config.Grid.ColLabels, testGridLabels)
	}
}

// TestReloadReresolvesGridLabels is the hazard the resolution introduces. The
// fallback used to be re-read on every use, so a characters change carried the
// labels with it for free; settling the labels once at load means a reload
// that changes characters has to settle them again, or the daemon keeps
// drawing a grid labeled from a character set the user has replaced.
func TestReloadReresolvesGridLabels(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	writeErr := os.WriteFile(configPath, []byte(gridConfigWithCharacters(testGridChars)), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	first := service.LoadWithValidation(configPath)
	if first.ValidationError != nil {
		t.Fatalf("first load failed: %v", first.ValidationError)
	}

	if first.Config.Grid.RowLabels != testGridLabels {
		t.Fatalf("Grid.RowLabels = %q, want %q", first.Config.Grid.RowLabels, testGridLabels)
	}

	rewriteErr := os.WriteFile(
		configPath,
		[]byte(gridConfigWithCharacters(testGridCharsAlt)),
		0o600,
	)
	if rewriteErr != nil {
		t.Fatalf("failed to rewrite config: %v", rewriteErr)
	}

	second := service.LoadWithValidation(configPath)
	if second.ValidationError != nil {
		t.Fatalf("second load failed: %v", second.ValidationError)
	}

	if second.Config.Grid.RowLabels != testGridLabelsAlt {
		t.Errorf(
			"after reload Grid.RowLabels = %q, want %q",
			second.Config.Grid.RowLabels, testGridLabelsAlt,
		)
	}

	if second.Config.Grid.ColLabels != testGridLabelsAlt {
		t.Errorf(
			"after reload Grid.ColLabels = %q, want %q",
			second.Config.Grid.ColLabels, testGridLabelsAlt,
		)
	}
}

// TestLoadResolvesGridLabelsAfterOverrides pins the ordering. The override
// file is layered after the config file is validated, so labels settled before
// that layer would be settled from characters the daemon does not run on —
// which is what `neru config set grid.characters` writes.
func TestLoadResolvesGridLabelsAfterOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	overridePath := loader.OverridePath(configPath)

	writeErr := os.WriteFile(configPath, []byte(gridConfigWithCharacters(testGridChars)), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	overrideErr := os.WriteFile(
		overridePath,
		[]byte(gridConfigWithCharacters("zxcv")),
		0o600,
	)
	if overrideErr != nil {
		t.Fatalf("failed to write override: %v", overrideErr)
	}

	service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(configPath)
	if result.ValidationError != nil {
		t.Fatalf("load failed: %v", result.ValidationError)
	}

	if result.Config.Grid.RowLabels != "ZXCV" {
		t.Errorf(
			"Grid.RowLabels = %q, want %q (the override's characters)",
			result.Config.Grid.RowLabels, "ZXCV",
		)
	}
}

// TestLoadKeepsConfiguredGridLabels guards the other direction: resolution
// fills the option in, it does not take it over.
func TestLoadKeepsConfiguredGridLabels(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	contents := gridConfigWithCharacters(testGridChars) + "row_labels = \"xy\"\n"

	writeErr := os.WriteFile(configPath, []byte(contents), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(configPath)
	if result.ValidationError != nil {
		t.Fatalf("load failed: %v", result.ValidationError)
	}

	if result.Config.Grid.RowLabels != "XY" {
		t.Errorf("Grid.RowLabels = %q, want %q", result.Config.Grid.RowLabels, "XY")
	}

	if result.Config.Grid.ColLabels != testGridLabels {
		t.Errorf("Grid.ColLabels = %q, want %q", result.Config.Grid.ColLabels, testGridLabels)
	}
}
