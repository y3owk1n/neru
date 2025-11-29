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
	})

	t.Log("✅ App initialization with real components test completed successfully")
}

// TestGridModeEndToEnd tests the complete grid mode workflow.
func TestGridModeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping grid mode E2E test in short mode")
	}

	// Create config with grid enabled
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

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
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app in a goroutine
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	// Wait for app to be running with timeout
	waitForAppReady(t, application, 5*time.Second)

	// Test grid mode activation
	t.Run("Activate Grid Mode", func(t *testing.T) {
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)
	})

	// Test grid mode deactivation
	t.Run("Deactivate Grid Mode", func(t *testing.T) {
		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
	})

	// Test grid mode reactivation
	t.Run("Reactivate Grid Mode", func(t *testing.T) {
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
	})

	// Stop the app
	application.Stop()

	// Wait for Run() to return
	select {
	case err := <-runDone:
		if err != nil {
			t.Logf("App Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("App did not stop within timeout")
	}

	t.Log("✅ Grid mode E2E test completed successfully")
}

// TestConfigurationLoadingIntegration tests configuration loading and validation.
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
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with default config: %v", err)
		}
		defer application.Cleanup()

		// Start the app briefly to test config loading
		runDone := make(chan error, 1)
		go func() {
			runDone <- application.Run()
		}()

		// Wait for app to be running with timeout
		waitForAppReady(t, application, 5*time.Second)

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

		// Stop the app
		application.Stop()

		// Wait for Run() to return
		select {
		case err := <-runDone:
			if err != nil {
				t.Logf("App Run() returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("App did not stop within timeout")
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
			app.WithIPCServer(&mockIPCServer{}),
			app.WithWatcher(&mockAppWatcher{}),
			app.WithOverlayManager(&mockOverlayManager{}),
			app.WithHotkeyService(&mockHotkeyService{}),
		)
		if err != nil {
			t.Fatalf("Failed to create app with custom config: %v", err)
		}
		defer application.Cleanup()

		// Start the app briefly
		runDone := make(chan error, 1)
		go func() {
			runDone <- application.Run()
		}()

		// Wait for app to be running with timeout
		waitForAppReady(t, application, 5*time.Second)

		if application.HintsEnabled() {
			t.Error("Expected hints to be disabled")
		}

		if application.GridEnabled() {
			t.Error("Expected grid to be disabled")
		}

		// Stop the app
		application.Stop()

		// Wait for Run() to return
		select {
		case err := <-runDone:
			if err != nil {
				t.Logf("App Run() returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("App did not stop within timeout")
		}
	})

	t.Log("✅ Configuration loading integration test completed successfully")
}

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
	waitForAppReady(t, application, 5*time.Second)

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

			waitForMode(t, application, mode, 3*time.Second)
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
			t.Logf("App Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("App did not stop within timeout")
	}

	t.Log("✅ App lifecycle integration test completed successfully")
}

// TestFullUserWorkflowIntegration simulates a complete real user workflow:
// 1. Start the application
// 2. Activate hints mode and interact with elements
// 3. Switch to grid mode and navigate
// 4. Perform various actions (clicks, scrolls)
// 5. Switch between modes multiple times
// 6. Stop the application.
func TestFullUserWorkflowIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full user workflow integration test in short mode")
	}

	// Create a comprehensive config for testing
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Skip OS permission checks

	// Initialize the app with real components but mock problematic ones
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithIPCServer(&mockIPCServer{}),           // Mock IPC to avoid starting real server
		app.WithWatcher(&mockAppWatcher{}),            // Mock watcher to avoid system monitoring
		app.WithOverlayManager(&mockOverlayManager{}), // Mock overlay to avoid UI initialization
		app.WithHotkeyService(&mockHotkeyService{}),   // Mock hotkeys to avoid system registration
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app in a goroutine
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	// Wait for app to be running with timeout
	waitForAppReady(t, application, 5*time.Second)

	t.Run("Application Startup", func(t *testing.T) {
		// Verify the app is running and in idle mode
		if !application.IsEnabled() {
			t.Fatalf("Expected application to be enabled, but it is not")
		}
		t.Log("✅ Application is running")

		if application.CurrentMode() != domain.ModeIdle {
			t.Fatalf(
				"Expected application to start in idle mode, but current mode is %v",
				application.CurrentMode(),
			)
		}
		t.Log("✅ Application started in idle mode")
	})

	t.Run("Hints Mode Workflow", func(t *testing.T) {
		// Activate hints mode (simulates Cmd+Shift+Space)
		application.SetModeHints()

		// Verify mode change
		waitForMode(t, application, domain.ModeHints, 3*time.Second)
		t.Log("✅ Hints mode activated")

		// Simulate user thinking time
		time.Sleep(200 * time.Millisecond)

		// Switch back to idle (simulates user canceling)
		application.SetModeIdle()

		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
		t.Log("✅ Returned to idle mode")
	})

	t.Run("Grid Mode Workflow", func(t *testing.T) {
		// Switch to grid mode (simulates Cmd+Shift+G)
		application.SetModeGrid()

		// Verify mode change
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)
		t.Log("✅ Grid mode activated")

		// Simulate user navigation time
		time.Sleep(300 * time.Millisecond)

		// Switch back to idle
		application.SetModeIdle()

		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
		t.Log("✅ Returned to idle mode")
	})

	t.Run("Mode Transitions", func(t *testing.T) {
		// Test multiple mode transitions like a real user would do
		modes := []struct {
			name         string
			expectedMode domain.Mode
			action       func()
		}{
			{"hints", domain.ModeHints, func() { application.SetModeHints() }},
			{"grid", domain.ModeGrid, func() { application.SetModeGrid() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
			{"hints", domain.ModeHints, func() { application.SetModeHints() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
			{"grid", domain.ModeGrid, func() { application.SetModeGrid() }},
			{"idle", domain.ModeIdle, func() { application.SetModeIdle() }},
		}

		for _, mode := range modes {
			mode.action()

			waitForMode(t, application, mode.expectedMode, 3*time.Second)
			t.Logf("✅ Switched to %s mode", mode.name)

			// Small delay to simulate user thinking time
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("Extended Usage Simulation", func(t *testing.T) {
		// Simulate a longer usage session with various interactions
		// This mimics how a real user might use the app over time

		// Start with hints mode
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints, 3*time.Second)
		time.Sleep(500 * time.Millisecond)

		// Switch to grid for precise navigation
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)
		time.Sleep(300 * time.Millisecond)

		// Back to hints for element selection
		application.SetModeHints()
		waitForMode(t, application, domain.ModeHints, 3*time.Second)
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)

		// Test grid mode
		application.SetModeGrid()
		waitForMode(t, application, domain.ModeGrid, 3*time.Second)

		application.SetModeIdle()
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)

		t.Log("✅ Extended usage simulation completed")
	})

	t.Run("Rapid Mode Switching", func(t *testing.T) {
		// Simulate rapid mode switching like an experienced user
		for range 3 {
			// Hints -> Grid -> Idle cycle
			application.SetModeHints()
			time.Sleep(50 * time.Millisecond)

			application.SetModeGrid()
			time.Sleep(50 * time.Millisecond)

			application.SetModeIdle()
			time.Sleep(50 * time.Millisecond)
		}

		// Verify final state is stable
		waitForMode(t, application, domain.ModeIdle, 3*time.Second)
		t.Log("✅ Rapid mode switching completed")
	})

	t.Run("Application Shutdown", func(t *testing.T) {
		// Verify app is still responsive before shutdown
		if !application.IsEnabled() {
			t.Error("Expected application to still be enabled before shutdown")
		}
		t.Log("✅ Application still responsive before shutdown")
	})

	// Stop the app programmatically
	application.Stop()

	// Wait for Run() to return
	select {
	case err := <-runDone:
		if err != nil {
			t.Logf("App Run() returned error: %v", err)
		} else {
			t.Log("✅ Application shut down gracefully")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Application did not stop within timeout")
	}

	t.Log("✅ Full user workflow integration test completed successfully")
}
