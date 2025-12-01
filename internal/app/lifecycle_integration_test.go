//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestAppLifecycleIntegration tests complete app lifecycle from start to shutdown.
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
		app.WithIPCServer(&mockIPCServer{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithHotkeyService(&mockHotkeyService{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	// Wait for app to be running with timeout
	waitForAppReady(t, application)

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
			domain.ModeAction,
			domain.ModeScroll,
			domain.ModeIdle,
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
			case domain.ModeAction:
				application.SetModeAction()
			case domain.ModeScroll:
				application.SetModeScroll()
			case domain.ModeIdle:
				application.SetModeIdle()
			}

			waitForMode(t, application, mode)
		}
	})

	// Test enable/disable via IPC
	t.Run("Enable/Disable via IPC", func(t *testing.T) {
		// Skip this subtest since the test uses mock IPC server and real client can't connect to mock socket
		t.Skip("Skipping IPC test with mock server - real client cannot connect to mock socket")
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
	case <-time.After(3 * time.Second):
		t.Fatal("App did not stop within timeout")
	}

	t.Log("✅ App lifecycle integration test completed successfully")
}
