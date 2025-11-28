package app_test

import (
	"context"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/action"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// Mock implementations for testing.
type mockEventTap struct{}

func (m *mockEventTap) Enable(_ context.Context) error  { return nil }
func (m *mockEventTap) Disable(_ context.Context) error { return nil }
func (m *mockEventTap) IsEnabled() bool                 { return false }
func (m *mockEventTap) SetHandler(_ func(string))       {}
func (m *mockEventTap) SetHotkeys(_ []string)           {}
func (m *mockEventTap) Destroy()                        {}

type mockIPCServer struct{}

func (m *mockIPCServer) Start(_ context.Context) error              { return nil }
func (m *mockIPCServer) Stop(_ context.Context) error               { return nil }
func (m *mockIPCServer) Serve(_ context.Context) error              { return nil }
func (m *mockIPCServer) Send(_ context.Context, _ any) (any, error) { return "", nil }
func (m *mockIPCServer) IsRunning() bool                            { return false }

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

type mockAppWatcher struct{}

func (m *mockAppWatcher) Start()                                {}
func (m *mockAppWatcher) Stop()                                 {}
func (m *mockAppWatcher) OnActivate(_ appwatcher.AppCallback)   {}
func (m *mockAppWatcher) OnDeactivate(_ appwatcher.AppCallback) {}
func (m *mockAppWatcher) OnTerminate(_ appwatcher.AppCallback)  {}
func (m *mockAppWatcher) OnScreenParametersChanged(_ func())    {}
