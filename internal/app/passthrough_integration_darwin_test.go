//go:build integration && darwin

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestExitAfterPassthroughIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping passthrough integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false
	cfg.General.PassthroughUnboundedKeys = true
	cfg.General.ShouldExitAfterPassthrough = true

	tap := &mockEventTap{}

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithEventTap(tap),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithHotkeyService(&mockHotkeyService{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	waitForAppReady(t, application)

	application.SetModeScroll()
	waitForMode(t, application, domain.ModeScroll)

	tap.triggerPassthrough()
	waitForMode(t, application, domain.ModeIdle)
}

func TestStalePassthroughCallbackDoesNotExitNewModeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping passthrough integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false
	cfg.General.PassthroughUnboundedKeys = true
	cfg.General.ShouldExitAfterPassthrough = true

	tap := &mockEventTap{}

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithEventTap(tap),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithHotkeyService(&mockHotkeyService{}),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	waitForAppReady(t, application)

	application.SetModeScroll()
	waitForMode(t, application, domain.ModeScroll)

	tap.mu.RLock()
	scrollCallback := tap.passthroughCallback
	tap.mu.RUnlock()

	if scrollCallback == nil {
		t.Fatal("expected scroll mode to register a passthrough callback")
	}

	application.SetModeGrid()
	waitForMode(t, application, domain.ModeGrid)

	scrollCallback()

	time.Sleep(50 * time.Millisecond)

	if application.CurrentMode() != domain.ModeGrid {
		t.Fatalf(
			"stale passthrough callback changed mode to %v, want grid",
			application.CurrentMode(),
		)
	}

	tap.triggerPassthrough()
	waitForMode(t, application, domain.ModeIdle)
}
