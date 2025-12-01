//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestGridModeEndToEnd tests the complete grid mode workflow.
func TestGridModeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping grid mode E2E test in short mode")
	}

	// Create config with grid enabled
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	// Initialize the app with real components but mock the problematic ones
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithIPCServer(&mockIPCServer{}),           // Mock IPC to avoid starting real server
		app.WithWatcher(&mockAppWatcher{}),            // Mock watcher to avoid system monitoring
		app.WithOverlayManager(&mockOverlayManager{}), // Mock overlay to avoid UI initialization
		app.WithHotkeyService(&mockHotkeyService{}),   // Mock hotkeys to avoid system registration
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app in a goroutine
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	// Wait for app to be running with timeout
	waitForAppReady(t, application)

	// Test grid mode activation
	t.Run("Activate Grid Mode", func(t *testing.T) {
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)
	})

	// Test grid mode deactivation
	t.Run("Deactivate Grid Mode", func(t *testing.T) {
		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Test grid mode reactivation
	t.Run("Reactivate Grid Mode", func(t *testing.T) {
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Stop the app
	application.Stop()

	// Wait for Run() to return
	select {
	case err := <-runDone:
		if err != nil {
			// Run() may return a context-canceled error after Stop(), which is expected
			t.Logf("App Run() returned (expected after Stop): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("App did not stop within timeout")
	}

	t.Log("✅ Grid mode E2E test completed successfully")
}
