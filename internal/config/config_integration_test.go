//go:build integration

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

// TestConfigFileOperationsIntegration tests real file system operations
// for configuration loading and reloading to prevent file system regressions.
func TestConfigFileOperationsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping config file operations integration test in short mode")
	}

	// Create a temporary directory for test config files
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.toml")

	t.Run("Config File Loading", func(t *testing.T) {
		// Create a test config file with custom settings
		configContent := `
[general]
accessibility_check_on_start = false
restore_cursor_position = true
excluded_apps = ["com.apple.finder", "com.test.app"]

[hints]
enabled = true
hint_characters = "asdfqwertzxcvb"
font_size = 14
font_family = "Monaco"
background_color = "#FFFF00"
text_color = "#000000"
opacity = 0.9

[grid]
enabled = true
font_size = 12
font_family = "Arial"
background_color = "#FFFFFF"
text_color = "#000000"
opacity = 0.8

[action]
enabled = true

[scroll]
enabled = true
scroll_amount = 3

[logging]
level = "debug"
file_path = "test.log"
max_size = 10
max_backups = 5
max_age = 30
`

		// Write config file to real file system
		err := os.WriteFile(configPath, []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write test config file: %v", err)
		}

		// Load config from real file
		service := config.NewService(config.DefaultConfig(), "", zap.NewNop())
		loadResult := service.LoadWithValidation(configPath)

		if loadResult.ValidationError != nil {
			t.Fatalf("Config validation failed: %v", loadResult.ValidationError)
		}

		// Verify the file was actually read and parsed correctly
		cfg := loadResult.Config
		if cfg.Hints.HintCharacters != "asdfqwertzxcvb" {
			t.Errorf(
				"Expected hint_characters 'asdfqwertzxcvb', got '%s'",
				cfg.Hints.HintCharacters,
			)
		}

		if cfg.Grid.FontSize != 12 {
			t.Errorf("Expected grid font_size 12, got %d", cfg.Grid.FontSize)
		}
	})

	t.Run("Config File Reloading", func(t *testing.T) {
		// Create initial config file
		initialContent := `
[hints]
font_size = 12
`

		err := os.WriteFile(configPath, []byte(initialContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write initial config: %v", err)
		}

		// Create a config service and load initial config
		configSvc := config.NewService(config.DefaultConfig(), configPath, zap.NewNop())

		initialLoad := configSvc.LoadWithValidation(configPath)
		if initialLoad.ValidationError != nil {
			t.Fatalf("Failed to load initial config: %v", initialLoad.ValidationError)
		}

		// Modify the real config file
		updatedContent := `
[hints]
font_size = 16
`

		err = os.WriteFile(configPath, []byte(updatedContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write updated config: %v", err)
		}

		// Reload config from the modified file
		err = configSvc.Reload(context.Background(), configPath)
		if err != nil {
			t.Fatalf("Failed to reload config: %v", err)
		}

		// Verify config was reloaded from the real file
		if configSvc.Get().Hints.FontSize != 16 {
			t.Errorf("Expected reloaded font_size to be 16, got %d", configSvc.Get().Hints.FontSize)
		}
	})

	t.Run("Config File Permissions", func(t *testing.T) {
		// Test that config files with restrictive permissions can still be read
		configContent := `
[hints]
enabled = true
`

		// Write with restrictive permissions
		err := os.WriteFile(configPath, []byte(configContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Should still be able to load it
		service := config.NewService(config.DefaultConfig(), "", zap.NewNop())

		loadResult := service.LoadWithValidation(configPath)
		if loadResult.ValidationError != nil {
			t.Fatalf(
				"Failed to load config with restrictive permissions: %v",
				loadResult.ValidationError,
			)
		}

		if !loadResult.Config.Hints.Enabled {
			t.Error("Expected hints to be enabled")
		}
	})

	t.Log("Config file operations integration test completed successfully")
}
