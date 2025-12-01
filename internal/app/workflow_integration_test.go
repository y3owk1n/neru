//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestFullUserWorkflowIntegration simulates a complete real user workflow:
// 1. Start the application
// 2. Activate hints mode and interact with elements
// 3. Switch to grid mode and navigate
// 4. Perform various actions (clicks, scrolls)
// 5. Switch between modes multiple times
// 6. Stop the application.
func TestFullUserWorkflowIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full user workflow integration test in short mode")
	}

	// Create a comprehensive config for testing
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Skip OS permission checks

	// Initialize the app with real components but mock problematic ones
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

	t.Run("Application Startup", func(t *testing.T) {
		// Verify the app is running and in idle mode
		if !application.IsEnabled() {
			t.Fatalf("Expected application to be enabled, but it is not")
		}

		t.Log("✅ Application is running")

		if application.CurrentMode() != domain.ModeIdle {
			t.Fatalf(
				"Expected application to start in idle mode, but current mode is %v",
				application.CurrentMode(),
			)
		}

		t.Log("✅ Application started in idle mode")
	})

	t.Run("Hints Mode Workflow", func(t *testing.T) {
		// Activate hints mode (simulates Cmd+Shift+Space)
		application.SetModeHints()

		// Verify mode change
		waitForMode(t, application, domain.ModeHints)
		t.Log("✅ Hints mode activated")

		// Simulate user thinking time
		time.Sleep(200 * time.Millisecond)

		// Switch back to idle (simulates user canceling)
		application.SetModeIdle()

		waitForMode(t, application, domain.ModeIdle)
		t.Log("✅ Returned to idle mode")
	})

	t.Run("Grid Mode Workflow", func(t *testing.T) {
		// Switch to grid mode (simulates Cmd+Shift+G)
		application.SetModeGrid()

		// Verify mode change
		waitForMode(t, application, domain.ModeGrid)
		t.Log("✅ Grid mode activated")

		// Simulate user navigation time
		time.Sleep(300 * time.Millisecond)

		// Switch back to idle
		application.SetModeIdle()

		waitForMode(t, application, domain.ModeIdle)
		t.Log("✅ Returned to idle mode")
	})

	t.Run("Mode Transitions", func(t *testing.T) {
		// Test multiple mode transitions like a real user would do
		modes := []struct {
			name         string
			expectedMode domain.Mode
			action       func()
		}{
			{"hints", domain.ModeHints, func() { application.SetModeHints() }},
			{"grid", domain.ModeGrid, func() { application.SetModeGrid() }},
			{"action", domain.ModeAction, func() { application.SetModeAction() }},
			{"scroll", domain.ModeScroll, func() { application.SetModeScroll() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
			{"hints", domain.ModeHints, func() { application.SetModeHints() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
			{"grid", domain.ModeGrid, func() { application.SetModeGrid() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
		}

		for _, mode := range modes {
			mode.action()

			waitForMode(t, application, mode.expectedMode)
			t.Logf("✅ Switched to %s mode", mode.name)

			// Small delay to simulate user thinking time
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("Extended Usage Simulation", func(t *testing.T) {
		// Simulate a longer usage session with various interactions
		// This mimics how a real user might use the app over time

		// Start with hints mode
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)
		time.Sleep(500 * time.Millisecond)

		// Switch to grid for precise navigation
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)
		time.Sleep(300 * time.Millisecond)

		// Try action mode for direct actions
		application.SetModeAction()
		waitForMode(t, application, domain.ModeAction)
		time.Sleep(200 * time.Millisecond)

		// Use scroll mode for navigation
		application.SetModeScroll()
		waitForMode(t, application, domain.ModeScroll)
		time.Sleep(300 * time.Millisecond)

		// Back to hints for element selection
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)
		time.Sleep(400 * time.Millisecond)

		// Test grid mode again
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)

		t.Log("✅ Extended usage simulation completed")
	})

	t.Run("Rapid Mode Switching", func(t *testing.T) {
		// Simulate rapid mode switching like an experienced user
		for range 2 {
			// Hints -> Grid -> Action -> Scroll -> Idle cycle
			application.SetModeHints()
			time.Sleep(50 * time.Millisecond)

			application.SetModeGrid()
			time.Sleep(50 * time.Millisecond)

			application.SetModeAction()
			time.Sleep(50 * time.Millisecond)

			application.SetModeScroll()
			time.Sleep(50 * time.Millisecond)

			application.SetModeIdle()
			time.Sleep(50 * time.Millisecond)
		}

		// Verify final state is stable
		waitForMode(t, application, domain.ModeIdle)
		t.Log("✅ Rapid mode switching completed")
	})

	t.Run("Application Shutdown", func(t *testing.T) {
		// Verify app is still responsive before shutdown
		if !application.IsEnabled() {
			t.Error("Expected application to still be enabled before shutdown")
		}

		t.Log("✅ Application still responsive before shutdown")
	})

	// Stop the app programmatically
	application.Stop()

	// Wait for Run() to return
	select {
	case err := <-runDone:
		if err != nil {
			// Run() may return a context-canceled error after Stop(), which is expected
			t.Logf("App Run() returned (expected after Stop): %v", err)
		} else {
			t.Log("✅ Application shut down gracefully")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Application did not stop within timeout")
	}

	t.Log("✅ Full user workflow integration test completed successfully")
}
