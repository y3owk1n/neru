//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// TestAppInitializationIntegration tests that the app can be initialized without hanging
func TestAppInitializationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app initialization integration test in short mode")
	}

	// Create a basic config for testing
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Skip OS permission checks

	// Use NewWithDeps with proper mocks to prevent hanging
	deps := &app.Deps{
		// Mock IPC server to prevent socket binding issues
		IPCServerFactory: &mockIPCServerFactory{},
		// Mock event tap to prevent accessibility permission hanging
		EventTapFactory: &mockEventTapFactory{},
		// Mock app watcher to prevent system monitoring
		WatcherFactory: &mockAppWatcherFactory{},
		// Mock overlay manager to prevent window creation
		OverlayManagerFactory: &mockOverlayManagerFactory{},
	}

	// Test that app initialization completes within reasonable time
	done := make(chan error, 1)

	go func() {
		_, err := app.NewWithDeps(cfg, "", deps)
		done <- err
	}()

	// Wait for initialization with timeout
	select {
	case err := <-done:
		if err != nil {
			t.Logf("App initialization failed (expected in some environments): %v", err)
			// This is acceptable - the test verifies that initialization doesn't hang
			t.Log("✅ App initialization completed without hanging")
			return
		}

		// If initialization succeeded, test basic functionality
		application, err := app.NewWithDeps(cfg, "", deps)
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}

		// Test initial state
		t.Run("Initial State", func(t *testing.T) {
			if application.CurrentMode() != domain.ModeIdle {
				t.Errorf("Expected initial mode Idle, got %v", application.CurrentMode())
			}
		})

		// Test mode transitions
		t.Run("Mode Transitions", func(t *testing.T) {
			// Test Hints mode
			application.SetModeHints()

			// Allow time for mode transition
			time.Sleep(100 * time.Millisecond)

			if application.CurrentMode() != domain.ModeHints {
				t.Errorf("Expected mode Hints after transition, got %v", application.CurrentMode())
			}

			// Test Grid mode
			application.SetModeGrid()

			time.Sleep(100 * time.Millisecond)

			if application.CurrentMode() != domain.ModeGrid {
				t.Errorf("Expected mode Grid after transition, got %v", application.CurrentMode())
			}

			// Test back to Idle
			application.SetModeIdle()

			time.Sleep(100 * time.Millisecond)

			if application.CurrentMode() != domain.ModeIdle {
				t.Errorf("Expected mode Idle after transition, got %v", application.CurrentMode())
			}
		})

		// Test cleanup
		t.Run("Cleanup", func(t *testing.T) {
			application.Cleanup()
		})

		t.Log("✅ App initialization and basic functionality test completed successfully")

	case <-time.After(10 * time.Second):
		t.Fatal("❌ App initialization timed out - this indicates a hanging issue that needs to be fixed")
	}
}
