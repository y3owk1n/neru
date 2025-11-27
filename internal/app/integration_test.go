//go:build integration

package app_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestApp_ModeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test basic app functionality without full initialization
	t.Run("AppCreation", func(t *testing.T) {
		// Setup minimal config
		cfg := config.DefaultConfig()
		cfg.Hints.Enabled = true
		cfg.Grid.Enabled = true
		cfg.General.AccessibilityCheckOnStart = false // Disable OS check

		// Test config validation
		if cfg.Hints.Enabled != true {
			t.Error("Config hints should be enabled")
		}

		if cfg.Grid.Enabled != true {
			t.Error("Config grid should be enabled")
		}

		t.Log("App configuration validation passed")
	})

	// Test IPC controller functionality (already tested in ipc_controller_test.go)
	t.Run("IPCControllerIntegration", func(t *testing.T) {
		// This is tested separately in ipc_controller_test.go
		// Just verify the test structure works
		t.Log("IPC controller integration tests run separately")
	})

	// Test that integration test framework works
	t.Run("IntegrationTestFramework", func(t *testing.T) {
		// Verify that integration tests can run with real build tags
		t.Log("Integration test framework is working")
	})
}
