//go:build unit

package app_test

import (
	"context"
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

// TestAppInitializationUnit tests app initialization logic with mocks.
func TestAppInitializationUnit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping app initialization unit test in short mode")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runTestAppInitializationUnit(t, ctx)
}

func runTestAppInitializationUnit(t *testing.T, ctx context.Context) {
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
	case <-ctx.Done():
		t.Fatal("App initialization timed out - possible hang")
	}

	// Test hint mode activation
	t.Run("Activate Hint Mode", func(t *testing.T) {
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints)

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
