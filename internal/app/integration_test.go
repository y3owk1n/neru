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

// Mock factories
type mockEventTapFactory struct{}

func (m *mockEventTapFactory) New(callback func(string), logger *zap.Logger) EventTap {
	return &mockEventTap{}
}

type mockEventTap struct{}

func (m *mockEventTap) Enable()                     {}
func (m *mockEventTap) Disable()                    {}
func (m *mockEventTap) Destroy()                    {}
func (m *mockEventTap) SetHotkeys(hotkeys []string) {}

type mockIPCServerFactory struct{}

func (m *mockIPCServerFactory) New(handler func(context.Context, ipc.Command) ipc.Response, logger *zap.Logger) (IPCServer, error) {
	return &mockIPCServer{}, nil
}

type mockIPCServer struct{}

func (m *mockIPCServer) Start()      {}
func (m *mockIPCServer) Stop() error { return nil }

type mockOverlayManagerFactory struct{}

func (m *mockOverlayManagerFactory) New(logger *zap.Logger) OverlayManager {
	return &mockOverlayManager{}
}

type mockOverlayManager struct{}

func (m *mockOverlayManager) Show()                                         {}
func (m *mockOverlayManager) Hide()                                         {}
func (m *mockOverlayManager) Clear()                                        {}
func (m *mockOverlayManager) ResizeToActiveScreenSync()                     {}
func (m *mockOverlayManager) SwitchTo(next overlay.Mode)                    {}
func (m *mockOverlayManager) Subscribe(fn func(overlay.StateChange)) uint64 { return 0 }
func (m *mockOverlayManager) Unsubscribe(id uint64)                         {}
func (m *mockOverlayManager) Destroy()                                      {}
func (m *mockOverlayManager) GetMode() overlay.Mode                         { return overlay.ModeIdle }
func (m *mockOverlayManager) GetWindowPtr() unsafe.Pointer                  { return nil }
func (m *mockOverlayManager) UseHintOverlay(o *hints.Overlay)               {}
func (m *mockOverlayManager) UseGridOverlay(o *grid.Overlay)                {}
func (m *mockOverlayManager) UseActionOverlay(o *action.Overlay)            {}
func (m *mockOverlayManager) UseScrollOverlay(o *scroll.Overlay)            {}
func (m *mockOverlayManager) GetHintOverlay() *hints.Overlay                { return nil }
func (m *mockOverlayManager) GetGridOverlay() *grid.Overlay                 { return nil }
func (m *mockOverlayManager) GetActionOverlay() *action.Overlay             { return nil }
func (m *mockOverlayManager) GetScrollOverlay() *scroll.Overlay             { return nil }
func (m *mockOverlayManager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	return nil
}
func (m *mockOverlayManager) DrawActionHighlight(x, y, w, h int) {}
func (m *mockOverlayManager) DrawScrollHighlight(x, y, w, h int) {}
func (m *mockOverlayManager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	return nil
}
func (m *mockOverlayManager) UpdateGridMatches(prefix string)                     {}
func (m *mockOverlayManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}
func (m *mockOverlayManager) SetHideUnmatched(hide bool)                          {}

type mockAppWatcherFactory struct{}

func (m *mockAppWatcherFactory) New(logger *zap.Logger) AppWatcher {
	return &mockAppWatcher{}
}

type mockAppWatcher struct{}

func (m *mockAppWatcher) Start()                                       {}
func (m *mockAppWatcher) Stop()                                        {}
func (m *mockAppWatcher) OnActivate(callback appwatcher.AppCallback)   {}
func (m *mockAppWatcher) OnDeactivate(callback appwatcher.AppCallback) {}
func (m *mockAppWatcher) OnTerminate(callback appwatcher.AppCallback)  {}
func (m *mockAppWatcher) OnScreenParametersChanged(callback func())    {}

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
		AppWatcherFactory:     &mockAppWatcherFactory{},
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
