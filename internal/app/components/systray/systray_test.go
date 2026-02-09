package systray_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app/components/systray"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap/zaptest"
)

// mockApp implements AppInterface for testing.
type mockApp struct {
	hintsEnabled    bool
	gridEnabled     bool
	quadGridEnabled bool
	isEnabled       bool
	activatedMode   domain.Mode
	configPath      string
	reloadCalled    bool
	cleanupCalled   bool
	enabledCallback func(bool)
}

func (m *mockApp) HintsEnabled() bool    { return m.hintsEnabled }
func (m *mockApp) GridEnabled() bool     { return m.gridEnabled }
func (m *mockApp) QuadGridEnabled() bool { return m.quadGridEnabled }
func (m *mockApp) IsEnabled() bool       { return m.isEnabled }
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
func (m *mockApp) Cleanup() { m.cleanupCalled = true }
func (m *mockApp) OnEnabledStateChanged(callback func(bool)) uint64 {
	m.enabledCallback = callback

	return 0
}

func TestNewComponent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockApp := &mockApp{}

	component := systray.NewComponent(mockApp, logger)

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

	component := systray.NewComponent(mockApp, logger)

	// OnReady should not panic and set up menu items
	// Note: We can't easily test systray.Run in unit tests
	// as it requires a GUI environment
	component.OnReady()

	// Test completed without panic - menu items are initialized internally
}

func TestComponent_OnExit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockApp := &mockApp{}

	component := systray.NewComponent(mockApp, logger)

	component.OnExit()

	if !mockApp.cleanupCalled {
		t.Error("Cleanup not called on exit")
	}
}
