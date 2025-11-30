//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestAppInitializationIntegration tests that the app can be initialized with real system components.
func TestAppInitializationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app initialization integration test in short mode")
	}

	// Create a basic config for testing
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Skip OS permission checks

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
		t.Fatalf("App initialization failed: %v", err)
	}
	defer application.Cleanup()

	// Test that the app initialized with real components
	if application == nil {
		t.Fatal("App should not be nil")
	}

	// Test basic app state
	if !application.IsEnabled() {
		t.Error("Expected app to be enabled initially")
	}

	if application.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode Idle, got %v", application.CurrentMode())
	}

	// Test mode transitions with real components
	t.Run("Mode Transitions with Real Components", func(t *testing.T) {
		// Test hints mode
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)

		// Test grid mode
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)

		// Test action mode
		application.SetModeAction()
		waitForMode(t, application, domain.ModeAction, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)

		// Test scroll mode
		application.SetModeScroll()
		waitForMode(t, application, domain.ModeScroll, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
	})

	t.Log("✅ App initialization with real components test completed successfully")
}
