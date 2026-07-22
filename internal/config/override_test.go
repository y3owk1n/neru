package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	testHintChar     = "qwerty"
	testHintsSection = "hints"
)

func TestOverridePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "default config path",
			path:     "/home/user/.config/neru/config.toml",
			expected: "/home/user/.config/neru/config.override.toml",
		},
		{
			name:     "custom config name",
			path:     "/home/user/.config/neru/my-neru.toml",
			expected: "/home/user/.config/neru/my-neru.override.toml",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path in current directory",
			path:     "neru.toml",
			expected: "neru.override.toml",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.OverridePath(testCase.path)
			if got != testCase.expected {
				t.Errorf("OverridePath(%q) = %q, want %q", testCase.path, got, testCase.expected)
			}
		})
	}
}

func TestSaveOverride(t *testing.T) {
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "config.override.toml")

	overrides := map[string]any{
		testHintsSection: map[string]any{
			"hint_characters": testHintChar,
		},
		"general": map[string]any{
			"passthrough_unbounded_keys": true,
		},
	}

	err := config.SaveOverride(overridePath, overrides)
	if err != nil {
		t.Fatalf("SaveOverride() failed: %v", err)
	}

	data, readErr := os.ReadFile(overridePath)
	if readErr != nil {
		t.Fatalf("Failed to read override file: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, `hint_characters = "`+testHintChar+`"`) {
		t.Errorf("Override file missing hint_characters, content:\n%s", content)
	}

	if !strings.Contains(content, "passthrough_unbounded_keys = true") {
		t.Errorf("Override file missing passthrough_unbounded_keys, content:\n%s", content)
	}

	if !strings.Contains(content, "# Neru runtime config overrides") {
		t.Errorf("Override file missing header comment, content:\n%s", content)
	}
}

func TestSaveOverrideEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "config.override.toml")

	err := config.SaveOverride(overridePath, nil)
	if err != nil {
		t.Fatalf("SaveOverride(nil) failed: %v", err)
	}

	_, statErr := os.Stat(overridePath)
	if !os.IsNotExist(statErr) {
		t.Errorf("SaveOverride(nil) should remove the file, but it still exists")
	}
}

func TestSaveOverrideRemovesOnEmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "config.override.toml")

	writeErr := os.WriteFile(overridePath, []byte("# old content"), 0o644)
	if writeErr != nil {
		t.Fatalf("Failed to write old content: %v", writeErr)
	}

	err := config.SaveOverride(overridePath, map[string]any{})
	if err != nil {
		t.Fatalf("SaveOverride(empty) failed: %v", err)
	}

	_, statErr := os.Stat(overridePath)
	if !os.IsNotExist(statErr) {
		t.Errorf("SaveOverride(empty) should remove the file, but it still exists")
	}
}

func TestService_SaveOverrideFieldAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	overridePath := config.OverridePath(configPath)

	configContent := `
[hints]
enabled = true
hint_characters = "asdfghjkl"
clickable_roles = ["AXButton"]
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(configPath)
	if result.ValidationError != nil {
		t.Fatalf("LoadWithValidation() failed: %v", result.ValidationError)
	}

	if result.Config.Hints.HintCharacters != "asdfghjkl" {
		t.Errorf("Expected hint_characters='asdfghjkl', got %q", result.Config.Hints.HintCharacters)
	}

	err := service.SaveOverrideField("hints.hint_characters", testHintChar)
	if err != nil {
		t.Fatalf("SaveOverrideField() failed: %v", err)
	}

	_, statErr := os.Stat(overridePath)
	if os.IsNotExist(statErr) {
		t.Fatal("Override file was not created")
	}

	data, readErr := os.ReadFile(overridePath)
	if readErr != nil {
		t.Fatalf("Failed to read override file: %v", readErr)
	}

	if !strings.Contains(string(data), `hint_characters = "`+testHintChar+`"`) {
		t.Errorf(
			"Override file should contain hint_characters = %q, content:\n%s",
			testHintChar,
			data,
		)
	}

	reloadedResult := service.LoadWithValidation(configPath)
	if reloadedResult.ValidationError != nil {
		t.Fatalf("Reload with override failed: %v", reloadedResult.ValidationError)
	}

	if reloadedResult.Config.Hints.HintCharacters != testHintChar {
		t.Errorf(
			"After override, expected hint_characters=%q, got %q",
			testHintChar,
			reloadedResult.Config.Hints.HintCharacters,
		)
	}
}

func TestService_SaveOverrideFieldNoConfigPath(t *testing.T) {
	service := config.NewService(config.DefaultConfig(), "", zap.NewNop(), nil)

	err := service.SaveOverrideField("hints.hint_characters", testHintChar)
	if err != nil {
		t.Errorf("SaveOverrideField() with empty path should return nil, got: %v", err)
	}
}

func TestService_OverridePreservesOtherFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[hints]
enabled = true
hint_characters = "asdfghjkl"
clickable_roles = ["AXButton"]

[general]
passthrough_unbounded_keys = false
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)
	_ = service.LoadWithValidation(configPath)

	_ = service.SaveOverrideField("hints.hint_characters", testHintChar)

	reloaded := service.LoadWithValidation(configPath)
	if reloaded.ValidationError != nil {
		t.Fatalf("Reload failed: %v", reloaded.ValidationError)
	}

	if reloaded.Config.Hints.HintCharacters != testHintChar {
		t.Errorf(
			"Expected hint_characters=%q, got %q",
			testHintChar,
			reloaded.Config.Hints.HintCharacters,
		)
	}

	if reloaded.Config.General.PassthroughUnboundedKeys != false {
		t.Errorf(
			"Expected passthrough_unbounded_keys=false, got %v",
			reloaded.Config.General.PassthroughUnboundedKeys,
		)
	}

	if len(reloaded.Config.Hints.ClickableRoles) != 1 ||
		reloaded.Config.Hints.ClickableRoles[0] != "AXButton" {
		t.Errorf(
			"Expected clickable_roles=[AXButton], got %v",
			reloaded.Config.Hints.ClickableRoles,
		)
	}
}

func TestService_OverrideInvalidatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[hints]
enabled = true
hint_characters = "asdfghjkl"
clickable_roles = ["AXButton"]
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)
	_ = service.LoadWithValidation(configPath)

	err := service.SaveOverrideField("hints.hint_characters", testHintChar)
	if err != nil {
		t.Fatalf("SaveOverrideField() failed: %v", err)
	}

	overridePath := config.OverridePath(configPath)
	badOverride := `
[hints]
hint_characters = "x"
`

	writeErr := os.WriteFile(overridePath, []byte(badOverride), 0o644)
	if writeErr != nil {
		t.Fatalf("Failed to write bad override: %v", writeErr)
	}

	result := service.LoadWithValidation(configPath)
	if result.ValidationError == nil {
		t.Error("Expected validation error with short hint_characters in override, got nil")
	}
}
