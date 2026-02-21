//go:build integration

package app_test

import (
	"context"
	"image"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// mockEventTap is a mock implementation of ports.EventTapPort for testing.
type mockEventTap struct {
	mu      sync.RWMutex
	enabled bool
	handler func(string)
}

// Enable implements ports.EventTapPort.
func (m *mockEventTap) Enable(_ context.Context) error {
	m.mu.Lock()
	m.enabled = true
	m.mu.Unlock()

	return nil
}

// Disable implements ports.EventTapPort.
func (m *mockEventTap) Disable(_ context.Context) error {
	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()

	return nil
}

// IsEnabled implements ports.EventTapPort.
func (m *mockEventTap) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.enabled
}

// SetHandler implements ports.EventTapPort.
func (m *mockEventTap) SetHandler(handler func(string)) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

// SetHotkeys implements ports.EventTapPort.
func (m *mockEventTap) SetHotkeys(_ []string) {}

// Destroy implements ports.EventTapPort.
func (m *mockEventTap) Destroy() {}

// mockIPCServer is a mock implementation of ports.IPCPort for testing.
type mockIPCServer struct {
	mu      sync.RWMutex
	running bool
}

// Start implements ports.IPCPort.
func (m *mockIPCServer) Start(_ context.Context) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	return nil
}

// Stop implements ports.IPCPort.
func (m *mockIPCServer) Stop(_ context.Context) error {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	return nil
}

// Send implements ports.IPCPort.
func (m *mockIPCServer) Send(_ context.Context, _ any) (any, error) { return "", nil }

// IsRunning implements ports.IPCPort.
func (m *mockIPCServer) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.running
}

// mockOverlayManager is a mock implementation of OverlayManager for testing.
type mockOverlayManager struct {
	mu        sync.RWMutex
	mode      overlay.Mode
	visible   bool
	subsCount uint64
}

// Show implements OverlayManager.
func (m *mockOverlayManager) Show() {
	m.mu.Lock()
	m.visible = true
	m.mu.Unlock()
}

// Hide implements OverlayManager.
func (m *mockOverlayManager) Hide() {
	m.mu.Lock()
	m.visible = false
	m.mu.Unlock()
}

// Clear implements OverlayManager.
func (m *mockOverlayManager) Clear() {
	m.mu.Lock()
	m.visible = false
	m.mu.Unlock()
}

// ResizeToActiveScreen implements OverlayManager.
func (m *mockOverlayManager) ResizeToActiveScreen() {}

// SwitchTo implements OverlayManager.
func (m *mockOverlayManager) SwitchTo(mode overlay.Mode) {
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
}

// Subscribe implements OverlayManager.
func (m *mockOverlayManager) Subscribe(
	_ func(overlay.StateChange),
) uint64 {
	m.mu.Lock()
	m.subsCount++
	count := m.subsCount
	m.mu.Unlock()

	return count
}

// Unsubscribe implements OverlayManager.
func (m *mockOverlayManager) Unsubscribe(_ uint64) {}

// Destroy implements OverlayManager.
func (m *mockOverlayManager) Destroy() {}

// Mode implements OverlayManager.
func (m *mockOverlayManager) Mode() overlay.Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mode
}

func (m *mockOverlayManager) WindowPtr() unsafe.Pointer                        { return nil }
func (m *mockOverlayManager) UseHintOverlay(_ *hints.Overlay)                  {}
func (m *mockOverlayManager) UseGridOverlay(_ *grid.Overlay)                   {}
func (m *mockOverlayManager) UseModeIndicatorOverlay(_ *scroll.Overlay)        {}
func (m *mockOverlayManager) UseRecursiveGridOverlay(_ *recursivegrid.Overlay) {}

func (m *mockOverlayManager) HintOverlay() *hints.Overlay { return nil }
func (m *mockOverlayManager) GridOverlay() *grid.Overlay  { return nil }
func (m *mockOverlayManager) ModeIndicatorOverlay() *scroll.Overlay {
	return nil
}

func (m *mockOverlayManager) RecursiveGridOverlay() *recursivegrid.Overlay {
	return nil
}

func (m *mockOverlayManager) DrawHintsWithStyle(
	_ []*hints.Hint,
	_ hints.StyleMode,
) error {
	return nil
}
func (m *mockOverlayManager) DrawModeIndicator(_, _ int) {}

func (m *mockOverlayManager) DrawGrid(
	_ *domainGrid.Grid,
	_ string,
	_ grid.Style,
) error {
	return nil
}

func (m *mockOverlayManager) DrawRecursiveGrid(
	_ image.Rectangle,
	_ int,
	_ string,
	_ int,
	_ int,
	_ recursivegrid.Style,
) error {
	return nil
}
func (m *mockOverlayManager) UpdateGridMatches(_ string)                   {}
func (m *mockOverlayManager) ShowSubgrid(_ *domainGrid.Cell, _ grid.Style) {}
func (m *mockOverlayManager) SetHideUnmatched(_ bool)                      {}
func (m *mockOverlayManager) SetSharingType(_ bool)                        {}

type mockHotkeyService struct {
	mu         sync.RWMutex
	registered map[string]hotkeys.Callback
	nextID     hotkeys.HotkeyID
}

func (m *mockHotkeyService) Register(
	key string,
	callback hotkeys.Callback,
) (hotkeys.HotkeyID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered == nil {
		m.registered = make(map[string]hotkeys.Callback)
	}

	m.registered[key] = callback
	id := m.nextID
	m.nextID++

	return id, nil
}

func (m *mockHotkeyService) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered != nil {
		m.registered = make(map[string]hotkeys.Callback)
	}

	m.nextID = 0
}

func (m *mockHotkeyService) GetRegisteredCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.registered)
}

type mockAppWatcher struct{}

func (m *mockAppWatcher) Start()                                {}
func (m *mockAppWatcher) Stop()                                 {}
func (m *mockAppWatcher) OnActivate(_ appwatcher.AppCallback)   {}
func (m *mockAppWatcher) OnDeactivate(_ appwatcher.AppCallback) {}
func (m *mockAppWatcher) OnTerminate(_ appwatcher.AppCallback)  {}
func (m *mockAppWatcher) OnScreenParametersChanged(_ func())    {}
