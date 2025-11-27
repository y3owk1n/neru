//go:build integration

package app_test

import (
	"context"
	"testing"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"github.com/y3owk1n/neru/internal/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Mock factories.
type mockEventTapFactory struct{}

func (m *mockEventTapFactory) New(_ func(string), _ *zap.Logger) app.EventTap {
	return &mockEventTap{}
}

type mockEventTap struct{}

func (m *mockEventTap) Enable()               {}
func (m *mockEventTap) Disable()              {}
func (m *mockEventTap) Destroy()              {}
func (m *mockEventTap) SetHotkeys(_ []string) {}

type mockIPCServerFactory struct{}

func (m *mockIPCServerFactory) New(
	_ func(context.Context, ipc.Command) ipc.Response,
	_ *zap.Logger,
) (app.IPCServer, error) {
	return &mockIPCServer{}, nil
}

type mockIPCServer struct{}

func (m *mockIPCServer) Start()      {}
func (m *mockIPCServer) Stop() error { return nil }

type mockOverlayManagerFactory struct{}

func (m *mockOverlayManagerFactory) New(_ *zap.Logger) app.OverlayManager {
	return &mockOverlayManager{}
}

type mockOverlayManager struct{}

func (m *mockOverlayManager) Show()                     {}
func (m *mockOverlayManager) Hide()                     {}
func (m *mockOverlayManager) Clear()                    {}
func (m *mockOverlayManager) ResizeToActiveScreenSync() {}
func (m *mockOverlayManager) SwitchTo(_ overlay.Mode)   {}

func (m *mockOverlayManager) Subscribe(
	_ func(overlay.StateChange),
) uint64 {
	return 0
}
func (m *mockOverlayManager) Unsubscribe(_ uint64) {}
func (m *mockOverlayManager) Destroy()             {}

func (m *mockOverlayManager) Mode() overlay.Mode { return overlay.ModeIdle }

func (m *mockOverlayManager) WindowPtr() unsafe.Pointer          { return nil }
func (m *mockOverlayManager) UseHintOverlay(_ *hints.Overlay)    {}
func (m *mockOverlayManager) UseGridOverlay(_ *grid.Overlay)     {}
func (m *mockOverlayManager) UseActionOverlay(_ *action.Overlay) {}
func (m *mockOverlayManager) UseScrollOverlay(_ *scroll.Overlay) {}

func (m *mockOverlayManager) HintOverlay() *hints.Overlay { return nil }

func (m *mockOverlayManager) GridOverlay() *grid.Overlay { return nil }

func (m *mockOverlayManager) ActionOverlay() *action.Overlay { return nil }

func (m *mockOverlayManager) ScrollOverlay() *scroll.Overlay { return nil }

func (m *mockOverlayManager) DrawHintsWithStyle(
	_ []*hints.Hint,
	_ hints.StyleMode,
) error {
	return nil
}
func (m *mockOverlayManager) DrawActionHighlight(_, _, _, _ int) {}
func (m *mockOverlayManager) DrawScrollHighlight(_, _, _, _ int) {}

func (m *mockOverlayManager) DrawGrid(
	_ *domainGrid.Grid,
	_ string,
	_ grid.Style,
) error {
	return nil
}
func (m *mockOverlayManager) UpdateGridMatches(_ string)                   {}
func (m *mockOverlayManager) ShowSubgrid(_ *domainGrid.Cell, _ grid.Style) {}
func (m *mockOverlayManager) SetHideUnmatched(_ bool)                      {}

type mockAppWatcherFactory struct{}

func (m *mockAppWatcherFactory) New(_ *zap.Logger) app.Watcher {
	return &mockAppWatcher{}
}

type mockAppWatcher struct{}

func (m *mockAppWatcher) Start()                                {}
func (m *mockAppWatcher) Stop()                                 {}
func (m *mockAppWatcher) OnActivate(_ appwatcher.AppCallback)   {}
func (m *mockAppWatcher) OnDeactivate(_ appwatcher.AppCallback) {}
func (m *mockAppWatcher) OnTerminate(_ appwatcher.AppCallback)  {}
func (m *mockAppWatcher) OnScreenParametersChanged(_ func())    {}

func TestApp_ModeIntegration(t *testing.T) {
	// Setup minimal config
	config := config.DefaultConfig()
	config.Hints.Enabled = true
	config.Grid.Enabled = true
	config.General.AccessibilityCheckOnStart = false // Disable OS check

	// Mock dependencies
	deps := &app.Deps{
		EventTapFactory:       &mockEventTapFactory{},
		IPCServerFactory:      &mockIPCServerFactory{},
		OverlayManagerFactory: &mockOverlayManagerFactory{},
		WatcherFactory:        &mockAppWatcherFactory{},
	}

	application, applicationErr := app.NewWithDeps(config, "", deps)
	if applicationErr != nil {
		t.Fatalf("Failed to create app: %v", applicationErr)
	}

	// Verify initial state
	if application.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode Idle, got %v", application.CurrentMode())
	}

	// Test getter methods
	if application.Config() != config {
		t.Error("Config() should return the config passed to NewWithDeps")
	}

	if application.Logger() == nil {
		t.Error("Logger() should return a valid logger")
	}

	if application.GetConfigPath() != "" {
		t.Errorf("GetConfigPath() should return empty string, got %q", application.GetConfigPath())
	}

	// Test feature flags
	if !application.HintsEnabled() {
		t.Error("HintsEnabled() should return true when hints are enabled in config")
	}

	if !application.GridEnabled() {
		t.Error("GridEnabled() should return true when grid is enabled in config")
	}

	if application.IsEnabled() != true {
		t.Error("IsEnabled() should return true for a properly initialized app")
	}

	// Test overlay manager access
	if application.OverlayManager() == nil {
		t.Error("OverlayManager() should return a valid overlay manager")
	}

	// Test context access
	if application.HintsContext() == nil {
		t.Error("HintsContext() should return a valid hints context")
	}

	if application.GridContext() == nil {
		t.Error("GridContext() should return a valid grid context")
	}

	if application.ScrollContext() == nil {
		t.Error("ScrollContext() should return a valid scroll context")
	}

	// Test event tap access
	if application.EventTap() == nil {
		t.Error("EventTap() should return a valid event tap")
	}

	// Test other methods that should not panic
	application.ExitMode()
	application.EnableEventTap()
	application.DisableEventTap()
	application.CaptureInitialCursorPosition()

	// Test ReloadConfig (should not panic even with invalid path)
	_ = application.ReloadConfig("/nonexistent/path.toml")
	// We don't check the error since it depends on file system, just that it doesn't panic

	// Test ActivateMode
	application.ActivateMode(domain.ModeHints)

	if application.CurrentMode() != domain.ModeHints {
		t.Errorf("ActivateMode should set mode to Hints, got %v", application.CurrentMode())
	}

	application.ActivateMode(domain.ModeGrid)

	if application.CurrentMode() != domain.ModeGrid {
		t.Errorf("ActivateMode should set mode to Grid, got %v", application.CurrentMode())
	}

	// Test mode persistence and switching
	application.SetModeHints()

	if application.CurrentMode() != domain.ModeHints {
		t.Errorf("SetModeHints should set mode to Hints, got %v", application.CurrentMode())
	}

	// Test that mode changes are logged
	t.Logf("Successfully switched to mode: %v", application.CurrentMode())

	// Test Mode Transitions with performance measurement
	start := time.Now()

	application.SetModeHints()

	hintsDuration := time.Since(start)

	if application.CurrentMode() != domain.ModeHints {
		t.Errorf("Expected mode Hints, got %v", application.CurrentMode())
	}

	t.Logf("Mode switch to Hints took %v", hintsDuration)

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
