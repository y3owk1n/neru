//go:build unit

package app_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestApp_ModeTransitions(t *testing.T) {
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

	// Cleanup
	application.Cleanup()
}
