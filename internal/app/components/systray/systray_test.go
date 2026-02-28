package systray_test

import (
	"context"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/app/components/systray"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap/zaptest"
)

// mockApp implements AppInterface for testing.
type mockApp struct {
	hintsEnabled         bool
	gridEnabled          bool
	recursiveGridEnabled bool
	isEnabled            bool
	activatedMode        domain.Mode
	configPath           string
	reloadCalled         bool
	cleanupCalled        bool
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
func (m *mockApp) Cleanup() { m.cleanupCalled = true }
func (m *mockApp) OnEnabledStateChanged(callback func(bool)) uint64 {
	m.enabledCallback = callback

	return 0
}
func (m *mockApp) OffEnabledStateChanged(id uint64) {}
func (m *mockApp) ToggleEnabled() {
	m.SetEnabled(!m.isEnabled)
}
func (m *mockApp) IsOverlayHiddenForScreenShare() bool      { return false }
func (m *mockApp) SetOverlayHiddenForScreenShare(hide bool) {}
func (m *mockApp) ToggleOverlayHiddenForScreenShare() bool {
	m.SetOverlayHiddenForScreenShare(!m.IsOverlayHiddenForScreenShare())

	return !m.IsOverlayHiddenForScreenShare()
}

func (m *mockApp) OnScreenShareStateChanged(callback func(bool)) uint64 {
	return 0
}
func (m *mockApp) OffScreenShareStateChanged(id uint64) {}

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

func TestDocsURLUsesVersionTagOrMain(t *testing.T) {
	testCases := []struct {
		name       string
		version    string
		path       string
		wantSuffix string
	}{
		{
			name:       "empty version falls back to main",
			version:    "",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "non semver falls back to main",
			version:    "dev",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "valid release tag",
			version:    "v1.19.0",
			path:       "docs/CLI.md",
			wantSuffix: "/v1.19.0/docs/CLI.md",
		},
		{
			name:       "git describe with commits",
			version:    "v1.19.0-3-gabcdef0",
			path:       "docs/CONFIGURATION.md",
			wantSuffix: "/v1.19.0/docs/CONFIGURATION.md",
		},
		{
			name:       "git describe dirty state",
			version:    "v1.19.0-dirty",
			path:       "docs/CLI.md",
			wantSuffix: "/v1.19.0/docs/CLI.md",
		},
		{
			name:       "invalid semver segments",
			version:    "v1.2",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "non numeric segment",
			version:    "v1.2.x",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			url := systray.DocsURL(testCase.path, testCase.version)
			if !strings.HasSuffix(url, testCase.wantSuffix) {
				t.Errorf("docs URL = %q, want suffix %q", url, testCase.wantSuffix)
			}
		})
	}
}
