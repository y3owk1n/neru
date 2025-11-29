//go:build !integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// initResult holds the result of app initialization for unit tests.
type initResult struct {
	app *app.App
	err error
}

// waitForMode waits for the application to reach the specified mode with a timeout.
func waitForMode(
	t *testing.T,
	application *app.App,
	expectedMode domain.Mode,
) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
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

// TestAppInitializationUnit tests app initialization logic with mocks.
func TestAppInitializationUnit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app initialization unit test in short mode")
	}

	// Add timeout to prevent hanging
	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()

		runTestAppInitializationUnit(t)
	}()

	select {
	case <-done:
		// Test completed normally
	case <-time.After(30 * time.Second):
		t.Fatal("TestAppInitializationUnit timed out - possible hang")
	}
}

func runTestAppInitializationUnit(t *testing.T) {
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

		application = res.app
		defer application.Cleanup()
	case <-time.After(5 * time.Second):
		t.Fatal("App initialization timed out - possible hang")
	}

	// Test hint mode activation
	t.Run("Activate Hint Mode", func(t *testing.T) {
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Test hint mode deactivation
	t.Run("Deactivate Hint Mode", func(t *testing.T) {
		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Test multiple mode transitions
	t.Run("Mode Transitions", func(t *testing.T) {
		// Test hints -> grid -> idle
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)

		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)

		// Test grid -> hints -> idle
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)

		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	t.Log("✅ App initialization unit test completed successfully")
}
