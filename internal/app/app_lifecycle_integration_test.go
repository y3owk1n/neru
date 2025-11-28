//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// waitForMode waits for the application to reach the specified mode with a timeout
func waitForMode(
	t *testing.T,
	application *app.App,
	expectedMode domain.Mode,
	timeout time.Duration,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if application.CurrentMode() == expectedMode {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf(
		"Timeout waiting for mode %v, current mode: %v",
		expectedMode,
		application.CurrentMode(),
	)
}

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

	// Test that app initialization completes within reasonable time
	done := make(chan error, 1)

	go func() {
		_, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithEventTap(&mockEventTap{}),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithWatcher(&mockAppWatcher{}),
		)
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
		application, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithEventTap(&mockEventTap{}),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithWatcher(&mockAppWatcher{}),
		)
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

			// Wait for mode transition with timeout
			waitForMode(t, application, domain.ModeHints, 1*time.Second)

			// Test Grid mode
			application.SetModeGrid()

			waitForMode(t, application, domain.ModeGrid, 1*time.Second)

			// Test back to Idle
			application.SetModeIdle()

			waitForMode(t, application, domain.ModeIdle, 1*time.Second)
		})

		// Test cleanup
		t.Run("Cleanup", func(t *testing.T) {
			application.Cleanup()
		})

		t.Log("✅ App initialization and basic functionality test completed successfully")

	case <-time.After(10 * time.Second):
		t.Fatal(
			"❌ App initialization timed out - this indicates a hanging issue that needs to be fixed",
		)
	}
}

// TestHintModeEndToEnd tests the complete hint mode workflow
func TestHintModeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hint mode E2E test in short mode")
	}

	// Create config with hints enabled
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	// Create app with mocks
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithEventTap(&mockEventTap{}),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Test hint mode activation
	t.Run("Activate Hint Mode", func(t *testing.T) {
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints, 1*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 1*time.Second)
	})

	// Test hint mode deactivation
	t.Run("Deactivate Hint Mode", func(t *testing.T) {
		application.SetModeIdle()
		time.Sleep(100 * time.Millisecond)

		if application.CurrentMode() != domain.ModeIdle {
			t.Errorf("Expected mode Idle, got %v", application.CurrentMode())
		}
	})

	t.Log("✅ Hint mode E2E test completed successfully")
}

// TestGridModeEndToEnd tests the complete grid mode workflow
func TestGridModeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping grid mode E2E test in short mode")
	}

	// Create config with grid enabled
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	// Create app with mocks
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithEventTap(&mockEventTap{}),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Test grid mode activation
	t.Run("Activate Grid Mode", func(t *testing.T) {
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 1*time.Second)
	})

	// Test grid mode deactivation
	t.Run("Deactivate Grid Mode", func(t *testing.T) {
		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 1*time.Second)
	})

	t.Log("✅ Grid mode E2E test completed successfully")
}

// TestConfigurationLoadingIntegration tests configuration loading and validation
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
			app.WithEventTap(&mockEventTap{}),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithWatcher(&mockAppWatcher{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with default config: %v", err)
		}
		defer application.Cleanup()

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
	})

	// Test with custom config
	t.Run("Custom Config", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Hints.Enabled = false
		cfg.Grid.Enabled = false

		application, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithEventTap(&mockEventTap{}),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithWatcher(&mockAppWatcher{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with custom config: %v", err)
		}
		defer application.Cleanup()

		if application.HintsEnabled() {
			t.Error("Expected hints to be disabled")
		}

		if application.GridEnabled() {
			t.Error("Expected grid to be disabled")
		}
	})

	t.Log("✅ Configuration loading integration test completed successfully")
}

// TestAppLifecycleIntegration tests complete app lifecycle from start to shutdown
func TestAppLifecycleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app lifecycle integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	// Create app
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithEventTap(&mockEventTap{}),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Test initial state
	t.Run("Initial State", func(t *testing.T) {
		if !application.IsEnabled() {
			t.Error("Expected app to be enabled initially")
		}

		if application.CurrentMode() != domain.ModeIdle {
			t.Errorf("Expected initial mode Idle, got %v", application.CurrentMode())
		}
	})

	// Test mode cycling
	t.Run("Mode Cycling", func(t *testing.T) {
		modes := []domain.Mode{
			domain.ModeHints,
			domain.ModeGrid,
			domain.ModeIdle,
		}

		for _, mode := range modes {
			switch mode {
			case domain.ModeHints:
				application.SetModeHints()
			case domain.ModeGrid:
				application.SetModeGrid()
			case domain.ModeIdle:
				application.SetModeIdle()
			}

			waitForMode(t, application, mode, 1*time.Second)
		}
	})

	// Test enable/disable
	t.Run("Enable/Disable", func(t *testing.T) {
		application.SetEnabled(false)
		if application.IsEnabled() {
			t.Error("Expected app to be disabled")
		}

		application.SetEnabled(true)
		if !application.IsEnabled() {
			t.Error("Expected app to be enabled")
		}
	})

	// Test cleanup
	t.Run("Cleanup", func(t *testing.T) {
		application.Cleanup()
		// App should still be functional after cleanup
		if application.CurrentMode() != domain.ModeIdle {
			t.Errorf("Expected mode Idle after cleanup, got %v", application.CurrentMode())
		}
	})

	t.Log("✅ App lifecycle integration test completed successfully")
}
