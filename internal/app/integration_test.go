package app

import (
	"context"
	"testing"
	"unsafe"

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

func (m *mockEventTapFactory) New(_ func(string), _ *zap.Logger) EventTap {
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
) (IPCServer, error) {
	return &mockIPCServer{}, nil
}

type mockIPCServer struct{}

func (m *mockIPCServer) Start()      {}
func (m *mockIPCServer) Stop() error { return nil }

type mockOverlayManagerFactory struct{}

func (m *mockOverlayManagerFactory) New(_ *zap.Logger) OverlayManager {
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

func (m *mockOverlayManager) GetMode() overlay.Mode { return overlay.ModeIdle }

func (m *mockOverlayManager) GetWindowPtr() unsafe.Pointer       { return nil }
func (m *mockOverlayManager) UseHintOverlay(_ *hints.Overlay)    {}
func (m *mockOverlayManager) UseGridOverlay(_ *grid.Overlay)     {}
func (m *mockOverlayManager) UseActionOverlay(_ *action.Overlay) {}
func (m *mockOverlayManager) UseScrollOverlay(_ *scroll.Overlay) {}

func (m *mockOverlayManager) GetHintOverlay() *hints.Overlay { return nil }

func (m *mockOverlayManager) GetGridOverlay() *grid.Overlay { return nil }

func (m *mockOverlayManager) GetActionOverlay() *action.Overlay { return nil }

func (m *mockOverlayManager) GetScrollOverlay() *scroll.Overlay { return nil }

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

func (m *mockAppWatcherFactory) New(_ *zap.Logger) Watcher {
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
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false // Disable OS check

	// Mock dependencies
	d := &deps{
		EventTapFactory:       &mockEventTapFactory{},
		IPCServerFactory:      &mockIPCServerFactory{},
		OverlayManagerFactory: &mockOverlayManagerFactory{},
		WatcherFactory:        &mockAppWatcherFactory{},
	}

	application, err := newWithDeps(cfg, "", d)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
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
