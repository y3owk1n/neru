//go:build integration

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// initResult holds the result of app initialization for integration tests.
type initResult struct {
	app *app.App
	err error
}

// TestAppInitializationIntegration tests app initialization logic with real components.
func TestAppInitializationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app initialization integration test in short mode")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runTestAppInitializationIntegration(t, ctx)
}

// setAndWaitForMode sets the application mode and waits for it to take effect
func setAndWaitForMode(t *testing.T, application *app.App, mode domain.Mode) {
	t.Helper()
	switch mode {
	case domain.ModeHints:
		application.SetModeHints()
	case domain.ModeGrid:
		application.SetModeGrid()
	case domain.ModeIdle:
		application.SetModeIdle()
	default:
		t.Fatalf("unknown mode: %v", mode)
	}
	waitForMode(t, application, mode, 3*time.Second)
}

func runTestAppInitializationIntegration(t *testing.T, ctx context.Context) {
	t.Helper()
	// Create a basic config for testing
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Skip OS permission checks

	// Test that app initialization completes within reasonable time
	done := make(chan initResult, 1)

	var application *app.App

	go func() {
		appInstance, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithEventTap(&mockEventTap{}),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		done <- initResult{app: appInstance, err: err}
	}()

	// Wait for initialization with timeout
	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("App initialization failed: %v", res.err)
		}
		if res.app == nil {
			t.Fatal("App initialization returned nil app without error")
		}

		application = res.app
		defer application.Cleanup()
	case <-ctx.Done():
		t.Fatal("App initialization timed out - possible hang")
	}

	// Test hint mode activation
	t.Run("Activate Hint Mode", func(t *testing.T) {
		setAndWaitForMode(t, application, domain.ModeHints)
		setAndWaitForMode(t, application, domain.ModeIdle)
	})

	// Test multiple mode transitions
	t.Run("Mode Transitions", func(t *testing.T) {
		// Test hints -> grid -> idle
		setAndWaitForMode(t, application, domain.ModeHints)
		setAndWaitForMode(t, application, domain.ModeGrid)
		setAndWaitForMode(t, application, domain.ModeIdle)

		// Test grid -> hints -> idle
		setAndWaitForMode(t, application, domain.ModeGrid)
		setAndWaitForMode(t, application, domain.ModeHints)
		setAndWaitForMode(t, application, domain.ModeIdle)
	})

	t.Log("✅ App initialization integration test completed successfully")
}

// TestApp_ModeTransitions tests app mode transitions with real components.
func TestApp_ModeTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app mode transitions integration test in short mode")
	}

	// Setup minimal config
	config := config.DefaultConfig()
	config.Hints.Enabled = true
	config.Grid.Enabled = true
	config.General.AccessibilityCheckOnStart = false // Disable OS check

	// Create app with mocked dependencies using functional options
	application, applicationErr := app.New(
		app.WithConfig(config),
		app.WithConfigPath(""),
		app.WithEventTap(&mockEventTap{}),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithHotkeyService(&mockHotkeyService{}),
	)
	if applicationErr != nil {
		t.Fatalf("Failed to create app: %v", applicationErr)
	}
	defer application.Cleanup()

	// Verify initial state
	if application.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode Idle, got %v", application.CurrentMode())
	}

	// Test Mode Transitions
	application.SetModeHints()

	if application.CurrentMode() != domain.ModeHints {
		t.Errorf("Expected mode Hints, got %v", application.CurrentMode())
	}

	application.SetModeGrid()

	if application.CurrentMode() != domain.ModeGrid {
		t.Errorf("Expected mode Grid, got %v", application.CurrentMode())
	}

	application.SetModeIdle()

	if application.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected mode Idle, got %v", application.CurrentMode())
	}
}
