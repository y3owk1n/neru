//go:build integration

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestService_Reload(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ["AXButton"]
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write temp config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath)

	// Test Reload
	ctx := context.Background()

	reloadErr := service.Reload(ctx, configPath)
	if reloadErr != nil {
		t.Fatalf("Reload() failed: %v", reloadErr)
	}

	cfg := service.Get()
	if cfg.Hints.HintCharacters != "asdf" {
		t.Errorf("Reload() did not load correct HintCharacters, got %v", cfg.Hints.HintCharacters)
	}
	if len(cfg.Hints.ClickableRoles) != 1 || cfg.Hints.ClickableRoles[0] != "AXButton" {
		t.Errorf("Reload() did not load correct ClickableRoles, got %v", cfg.Hints.ClickableRoles)
	}

	// Test Reload with invalid file
	anotherWriteFileErr := os.WriteFile(configPath, []byte("invalid toml content"), 0o644)
	if anotherWriteFileErr != nil {
		t.Fatalf("Failed to update temp config: %v", anotherWriteFileErr)
	}

	anotherReloadErr := service.Reload(ctx, configPath)
	if anotherReloadErr == nil {
		t.Error("Reload() should fail with invalid config file")
	}
}
