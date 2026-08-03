//go:build integration

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

func TestService_Reload(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ["button"]
`

	writeFileErr := os.WriteFile(configPath, []byte(configContent), 0o644)
	if writeFileErr != nil {
		t.Fatalf("Failed to write temp config: %v", writeFileErr)
	}

	service := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

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

	if len(cfg.Hints.ClickableRoles) != 1 || cfg.Hints.ClickableRoles[0] != TestRoleButton {
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

// TestService_ReloadWithRoleVocabulary covers config reload across the role
// vocabulary: a valid change takes effect, and an invalid one is rejected
// without disturbing the config the daemon is already running with.
func TestService_ReloadWithRoleVocabulary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	write := func(t *testing.T, roles string) {
		t.Helper()

		contents := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ` + roles + `

[[hints.app_configs]]
bundle_id = "com.example.app"
additional_clickable_roles = ["heading"]
`

		err := os.WriteFile(configPath, []byte(contents), 0o600)
		if err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
	}

	write(t, `["button", "link"]`)

	svc := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)
	ctx := context.Background()

	err := svc.Reload(ctx, configPath)
	if err != nil {
		t.Fatalf("initial Reload() failed: %v", err)
	}

	before := svc.Get().ClickableRolesForApp("com.example.app")
	if len(before) == 0 {
		t.Fatal("ClickableRolesForApp() resolved to nothing after reload")
	}

	// A valid reload must take effect, including the app-specific addition.
	write(t, `["button", "link", "checkbox"]`)

	err = svc.Reload(ctx, configPath)
	if err != nil {
		t.Fatalf("second Reload() failed: %v", err)
	}

	after := svc.Get().ClickableRolesForApp("com.example.app")
	if len(after) <= len(before) {
		t.Errorf(
			"ClickableRolesForApp() = %v after adding checkbox, want more than %v",
			after, before,
		)
	}

	for _, want := range element.ResolveRolesForCurrentPlatform([]string{"checkbox"}).Native {
		if !slices.Contains(after, want) {
			t.Errorf("ClickableRolesForApp() = %v, missing newly added role %q", after, want)
		}
	}

	// An invalid reload must be rejected and leave the running config alone.
	write(t, `["AXButton"]`)

	err = svc.Reload(ctx, configPath)
	if err == nil {
		t.Fatal("Reload() accepted a config using the retired role vocabulary")
	}

	if !strings.Contains(err.Error(), `use "button"`) {
		t.Errorf("Reload() error = %v, want a migration hint naming \"button\"", err)
	}

	survived := svc.Get().ClickableRolesForApp("com.example.app")
	if !slices.Equal(survived, after) {
		t.Errorf(
			"rejected reload changed the running config: got %v, want %v",
			survived, after,
		)
	}
}

// TestService_ReloadRejectsInvalidAppConfigRoles pins validation of the
// per-app role list, which is merged in before resolution.
func TestService_ReloadRejectsInvalidAppConfigRoles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	contents := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ["button"]

[[hints.app_configs]]
bundle_id = "com.example.app"
additional_clickable_roles = ["AXHeading"]
`

	err := os.WriteFile(configPath, []byte(contents), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	svc := config.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

	err = svc.Reload(context.Background(), configPath)
	if err == nil {
		t.Fatal("Reload() accepted an app config using the retired role vocabulary")
	}

	if !strings.Contains(err.Error(), "additional_clickable_roles") {
		t.Errorf("Reload() error = %v, want it to name the offending field", err)
	}
}
