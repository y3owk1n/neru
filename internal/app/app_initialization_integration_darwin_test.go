//go:build integration && darwin

package app_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// TestAppInitializationWithRealComponentsIntegration tests that the app can be initialized with real system components.
func TestAppInitializationWithRealComponentsIntegration(t *testing.T) {
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
		waitForMode(t, application, domain.ModeHints)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)

		// Test grid mode
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)

		// Test scroll mode
		application.SetModeScroll()
		waitForMode(t, application, domain.ModeScroll)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle)
	})

	// Test that Escape exits a mode via per-mode custom hotkeys defaults.
	t.Run("Escape Exits Mode Via CustomHotkeys", func(t *testing.T) {
		cfg2 := config.DefaultConfig()
		cfg2.Hints.Enabled = true
		cfg2.Grid.Enabled = false
		cfg2.General.AccessibilityCheckOnStart = false

		application2, err := app.New(
			app.WithConfig(cfg2),
			app.WithConfigPath(""),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("App initialization failed: %v", err)
		}
		defer application2.Cleanup()

		waitForAppReady(t, application2)
		application2.SetModeHints()
		waitForMode(t, application2, domain.ModeHints)
		application2.HandleKeyPress("Escape")
		waitForMode(t, application2, domain.ModeIdle)
	})

	t.Log("✅ App initialization with real components test completed successfully")
}

func TestAppInitialization_Systray(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping systray initialization test in short mode")
	}

	t.Run("Systray Enabled (Default)", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Systray.Enabled = true // Explicitly set, though default is true
		cfg.General.AccessibilityCheckOnStart = false

		appInstance, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("App initialization failed: %v", err)
		}
		defer appInstance.Cleanup()

		if appInstance.GetSystrayComponent() == nil {
			t.Error("Expected systray component to be initialized when enabled")
		}
	})

	t.Run("Systray Disabled", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Systray.Enabled = false
		cfg.General.AccessibilityCheckOnStart = false

		appInstance, err := app.New(
			app.WithConfig(cfg),
			app.WithConfigPath(""),
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("App initialization failed: %v", err)
		}
		defer appInstance.Cleanup()

		if appInstance.GetSystrayComponent() != nil {
			t.Error("Expected systray component to be nil when disabled")
		}
	})
}
