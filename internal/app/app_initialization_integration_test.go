//go:build integration

package app_test

import (
	"os"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
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

	// Test that configured exit keys are respected end-to-end
	t.Run("Exit Key From Config Integration", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping exit-key integration test in short mode")
		}

		cfg2 := config.DefaultConfig()
		cfg2.Hints.Enabled = true
		cfg2.Grid.Enabled = false
		cfg2.General.AccessibilityCheckOnStart = false
		// Configure a custom exit key (modifier combo) and verify it exits modes
		cfg2.General.ModeExitKeys = []string{"Ctrl+C"}

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

		// Activate hints mode then send the configured exit key
		application2.SetModeHints()
		waitForMode(t, application2, domain.ModeHints)

		application2.HandleKeyPress("Ctrl+C")
		waitForMode(t, application2, domain.ModeIdle)
	})

	// Test that loading a config file which only contains Ctrl+C as exit key
	// prevents Escape from exiting modes.
	t.Run("Config File Exit Keys Override Integration", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping config-file exit-key integration test in short mode")
		}

		// Create a temporary config file that only specifies Ctrl+C as the exit key
		tmpf, err := os.CreateTemp(t.TempDir(), "neru-config-*.toml")
		if err != nil {
			t.Fatalf("failed to create temp config: %v", err)
		}

		defer func() {
			_ = tmpf.Close()
			_ = os.Remove(tmpf.Name())
		}()

		cfgContent := `[general]
	mode_exit_keys = ["Ctrl+C"]
	[hints]
	enabled = true
	`

		_, err = tmpf.WriteString(cfgContent)
		if err != nil {
			t.Fatalf("failed to write temp config: %v", err)
		}

		err = tmpf.Sync()
		if err != nil {
			t.Fatalf("failed to sync temp config: %v", err)
		}

		// Load the config via the same path logic used by the CLI
		svc := config.NewService(config.DefaultConfig(), "", zap.NewNop())

		load := svc.LoadWithValidation(tmpf.Name())
		if load.ValidationError != nil {
			t.Fatalf("config validation failed: %v", load.ValidationError)
		}

		// Ensure the loaded config contains only the configured exit key
		if len(load.Config.General.ModeExitKeys) != 1 ||
			load.Config.General.ModeExitKeys[0] != "Ctrl+C" {
			t.Fatalf(
				"unexpected ModeExitKeys after loading config: %v",
				load.Config.General.ModeExitKeys,
			)
		}

		application2, err := app.New(
			app.WithConfig(load.Config),
			app.WithConfigPath(load.ConfigPath),
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

		// Activate hints mode
		application2.SetModeHints()
		waitForMode(t, application2, domain.ModeHints)

		// Sending Escape (as raw byte) should NOT exit
		application2.HandleKeyPress("\x1b")
		time.Sleep(50 * time.Millisecond)

		if application2.CurrentMode() != domain.ModeHints {
			t.Fatalf("raw escape unexpectedly exited mode when config restricted exit keys")
		}

		// Also try the named representation
		application2.HandleKeyPress("escape")
		time.Sleep(50 * time.Millisecond)

		if application2.CurrentMode() != domain.ModeHints {
			t.Fatalf("named escape unexpectedly exited mode when config restricted exit keys")
		}

		// Now send the configured exit key
		application2.HandleKeyPress("Ctrl+C")
		waitForMode(t, application2, domain.ModeIdle)
	})

	t.Log("✅ App initialization with real components test completed successfully")
}
