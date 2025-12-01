//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestScrollModeEndToEnd tests the complete scroll mode workflow.
func TestScrollModeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scroll mode E2E test in short mode")
	}

	// Create config with scroll mode enabled (scroll mode doesn't have a specific enable flag)
	cfg := config.DefaultConfig()
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

	// Test scroll mode activation
	t.Run("Activate Scroll Mode", func(t *testing.T) {
		application.SetModeScroll()
		waitForMode(t, application, domain.ModeScroll)
	})

	// Test scroll mode deactivation
	t.Run("Deactivate Scroll Mode", func(t *testing.T) {
		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Test scroll mode reactivation
	t.Run("Reactivate Scroll Mode", func(t *testing.T) {
		application.SetModeScroll()
		waitForMode(t, application, domain.ModeScroll)

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

	t.Log("✅ Scroll mode E2E test completed successfully")
}
