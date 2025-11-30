//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
)

// TestHotkeyIntegration tests hotkey registration during app lifecycle
func TestHotkeyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hotkey integration test in short mode")
	}

	// Create a mock hotkey service that tracks registrations
	mockHotkeys := &mockHotkeyService{}

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithHotkeyService(mockHotkeys),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app to trigger hotkey registration
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()
	waitForAppReady(t, application, 5*time.Second)

	// Poll for hotkey registration with timeout
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mockHotkeys.GetRegisteredCount() >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Test that hotkeys are registered during app startup
	t.Run("Hotkey Registration", func(t *testing.T) {
		// Hotkeys should be registered during app startup.
		// DefaultConfig defines 3 bindings: hints, grid, and scroll
		registeredCount := mockHotkeys.GetRegisteredCount()
		if registeredCount == 0 {
			t.Error("Expected hotkeys to be registered during app startup")
		}
		// Verify we have at least the 3 default hotkeys from config.DefaultConfig().Hotkeys.Bindings
		if registeredCount < 3 {
			t.Errorf("Expected at least 3 hotkeys registered, got %d", registeredCount)
		}
	})

	application.Stop()
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("App did not stop within timeout")
	}

	t.Log("✅ Hotkey integration test completed successfully")
}
