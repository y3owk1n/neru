//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
)

// TestConfigurationLoadingIntegration tests configuration loading and application of settings.
func TestConfigurationLoadingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping configuration loading integration test in short mode")
	}

	// Test with default config
	t.Run("Default Config", func(t *testing.T) {
		cfg := config.DefaultConfig()

		application, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with default config: %v", err)
		}
		defer application.Cleanup()

		// Start the app briefly to test config loading
		runDone := make(chan error, 1)
		go func() {
			runDone <- application.Run()
		}()

		// Wait for app to be running with timeout
		waitForAppReady(t, application)

		// Verify config is properly set
		if application.Config() == nil {
			t.Error("Expected config to be set")
		}

		if !application.HintsEnabled() {
			t.Error("Expected hints to be enabled by default")
		}

		if !application.GridEnabled() {
			t.Error("Expected grid to be enabled by default")
		}

		// Stop the app
		application.Stop()

		// Wait for Run() to return
		select {
		case err := <-runDone:
			if err != nil {
				t.Logf("App Run() returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("App did not stop within timeout")
		}
	})

	// Test with custom config
	t.Run("Custom Config", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Hints.Enabled = false
		cfg.Grid.Enabled = false

		application, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with custom config: %v", err)
		}
		defer application.Cleanup()

		// Start the app briefly
		runDone := make(chan error, 1)
		go func() {
			runDone <- application.Run()
		}()

		// Wait for app to be running with timeout
		waitForAppReady(t, application)

		if application.HintsEnabled() {
			t.Error("Expected hints to be disabled")
		}

		if application.GridEnabled() {
			t.Error("Expected grid to be disabled")
		}

		// Stop the app
		application.Stop()

		// Wait for Run() to return
		select {
		case err := <-runDone:
			if err != nil {
				// Run() may return a context-canceled error after Stop(), which is expected
				t.Logf("App Run() returned (expected after Stop): %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("App did not stop within timeout")
		}
	})

	t.Log("✅ Configuration loading integration test completed successfully")
}
