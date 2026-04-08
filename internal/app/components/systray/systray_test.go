package systray_test

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/y3owk1n/neru/internal/app/components/systray"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// mockApp implements AppInterface for testing.
type mockApp struct {
	hintsEnabled         bool
	gridEnabled          bool
	recursiveGridEnabled bool
	isEnabled            bool
	hiddenForScreenShare bool
	activatedMode        domain.Mode
	configPath           string
	reloadCalled         bool
	enabledCallback      func(bool)
}

func (m *mockApp) HintsEnabled() bool         { return m.hintsEnabled }
func (m *mockApp) GridEnabled() bool          { return m.gridEnabled }
func (m *mockApp) RecursiveGridEnabled() bool { return m.recursiveGridEnabled }
func (m *mockApp) IsEnabled() bool            { return m.isEnabled }
func (m *mockApp) SetEnabled(enabled bool) {
	m.isEnabled = enabled
	if m.enabledCallback != nil {
		m.enabledCallback(enabled)
	}
}
func (m *mockApp) ActivateMode(mode domain.Mode) { m.activatedMode = mode }
func (m *mockApp) GetConfigPath() string         { return m.configPath }
func (m *mockApp) ReloadConfig(ctx context.Context, configPath string) error {
	m.reloadCalled = true

	return nil
}

func (m *mockApp) OnEnabledStateChanged(callback func(bool)) uint64 {
	m.enabledCallback = callback

	return 0
}
func (m *mockApp) OffEnabledStateChanged(id uint64) {}
func (m *mockApp) ToggleEnabled() {
	m.SetEnabled(!m.isEnabled)
}
func (m *mockApp) IsOverlayHiddenForScreenShare() bool      { return m.hiddenForScreenShare }
func (m *mockApp) SetOverlayHiddenForScreenShare(hide bool) { m.hiddenForScreenShare = hide }
func (m *mockApp) ToggleOverlayHiddenForScreenShare() bool {
	newState := !m.hiddenForScreenShare
	m.hiddenForScreenShare = newState

	return newState
}

func (m *mockApp) OnScreenShareStateChanged(callback func(bool)) uint64 {
	return 0
}
func (m *mockApp) OffScreenShareStateChanged(id uint64) {}

func TestNewComponent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockApp := &mockApp{}

	component := systray.NewComponent(mockApp, nil, logger)

	if component == nil {
		t.Fatal("NewComponent returned nil")
	}
}

func TestComponent_OnReady(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockApp := &mockApp{
		hintsEnabled: true,
		gridEnabled:  true,
	}

	component := systray.NewComponent(mockApp, nil, logger)

	// OnReady should not panic and set up menu items
	// Note: We can't easily test systray.Run in unit tests
	// as it requires a GUI environment
	component.OnReady()

	// Test completed without panic - menu items are initialized internally
}

func TestComponent_OnExit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockApp := &mockApp{}

	component := systray.NewComponent(mockApp, nil, logger)

	// OnExit should not panic; cleanup is owned by the daemon host, not the
	// systray component.
	component.OnExit()
}
