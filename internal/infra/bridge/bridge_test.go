package bridge_test

import (
	"testing"
	"unsafe"

	"github.com/y3owk1n/neru/internal/infra/bridge"
	"go.uber.org/zap"
)

// MockAppWatcher implements AppWatcher for testing.
type MockAppWatcher struct {
	launchCalls       []AppEvent
	terminateCalls    []AppEvent
	activateCalls     []AppEvent
	deactivateCalls   []AppEvent
	screenChangeCalls int
}

// AppEvent represents an app event.
type AppEvent struct {
	appName  string
	bundleID string
}

func (m *MockAppWatcher) HandleLaunch(appName, bundleID string) {
	m.launchCalls = append(m.launchCalls, AppEvent{appName, bundleID})
}

func (m *MockAppWatcher) HandleTerminate(appName, bundleID string) {
	m.terminateCalls = append(m.terminateCalls, AppEvent{appName, bundleID})
}

func (m *MockAppWatcher) HandleActivate(appName, bundleID string) {
	m.activateCalls = append(m.activateCalls, AppEvent{appName, bundleID})
}

func (m *MockAppWatcher) HandleDeactivate(appName, bundleID string) {
	m.deactivateCalls = append(m.deactivateCalls, AppEvent{appName, bundleID})
}

func (m *MockAppWatcher) HandleScreenParametersChanged() {
	m.screenChangeCalls++
}

func TestInitializeLogger(t *testing.T) {
	tests := []struct {
		name   string
		logger *zap.Logger
	}{
		{
			name:   "initialize with development logger",
			logger: zap.NewNop(),
		},
		{
			name:   "initialize with nil logger",
			logger: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Should not panic
			bridge.InitializeLogger(testCase.logger)

			// Verify logger was set
			if testCase.logger != nil && bridge.Logger() == nil {
				t.Error("Expected logger to be set")
			}
		})
	}
}

func TestSetAppWatcher(t *testing.T) {
	tests := []struct {
		name    string
		watcher bridge.AppWatcherInterface
	}{
		{
			name:    "set mock watcher",
			watcher: &MockAppWatcher{},
		},
		{
			name:    "set nil watcher",
			watcher: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Initialize logger for testing
			bridge.InitializeLogger(zap.NewNop())

			// Should not panic
			bridge.SetAppWatcher(testCase.watcher)

			// Verify watcher was set
			if testCase.watcher != nil && bridge.AppWatcher() == nil {
				t.Error("Expected watcher to be set")
			}
		})
	}
}

func TestCallbacks(t *testing.T) {
	// Initialize logger
	bridge.InitializeLogger(zap.NewNop())

	// Setup mock watcher
	mock := &MockAppWatcher{}
	bridge.SetAppWatcher(mock)

	t.Run("HandleAppLaunch", func(t *testing.T) {
		bridge.HandleAppLaunch("TestApp", "com.test.app")

		if len(mock.launchCalls) != 1 {
			t.Errorf("Expected 1 launch call, got %d", len(mock.launchCalls))
		}

		if mock.launchCalls[0].appName != "TestApp" {
			t.Errorf("Expected app name 'TestApp', got '%s'", mock.launchCalls[0].appName)
		}

		if mock.launchCalls[0].bundleID != "com.test.app" {
			t.Errorf("Expected bundle ID 'com.test.app', got '%s'", mock.launchCalls[0].bundleID)
		}
	})

	t.Run("HandleAppTerminate", func(t *testing.T) {
		bridge.HandleAppTerminate("TestApp", "com.test.app")

		if len(mock.terminateCalls) != 1 {
			t.Errorf("Expected 1 terminate call, got %d", len(mock.terminateCalls))
		}
	})

	t.Run("HandleAppActivate", func(t *testing.T) {
		bridge.HandleAppActivate("TestApp", "com.test.app")

		if len(mock.activateCalls) != 1 {
			t.Errorf("Expected 1 activate call, got %d", len(mock.activateCalls))
		}
	})

	t.Run("HandleAppDeactivate", func(t *testing.T) {
		bridge.HandleAppDeactivate("TestApp", "com.test.app")

		if len(mock.deactivateCalls) != 1 {
			t.Errorf("Expected 1 deactivate call, got %d", len(mock.deactivateCalls))
		}
	})

	t.Run("HandleScreenParametersChanged", func(_ *testing.T) {
		bridge.HandleScreenParametersChanged()
		// Since it runs in a goroutine, we need to wait a bit
		// But for unit test reliability, we might just check if it didn't panic
		// or use a channel in mock to sync.
		// For now, let's just ensure it doesn't panic.
	})
}

func TestHasClickAction(t *testing.T) {
	tests := []struct {
		name    string
		element unsafe.Pointer
		want    bool
	}{
		{
			name:    "nil element returns false",
			element: nil,
			want:    false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Initialize logger for testing
			bridge.InitializeLogger(zap.NewNop())

			got := bridge.HasClickAction(testCase.element)
			if got != testCase.want {
				t.Errorf("HasClickAction() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestActiveScreenBounds(t *testing.T) {
	// Initialize logger for testing
	bridge.InitializeLogger(zap.NewNop())

	bounds := bridge.ActiveScreenBounds()

	// Verify bounds are valid (non-zero)
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("ActiveScreenBounds() returned invalid bounds: %v", bounds)
	}

	// Verify bounds are reasonable (not negative, not absurdly large)
	if bounds.Min.X < -10000 || bounds.Min.Y < -10000 {
		t.Errorf("ActiveScreenBounds() returned unreasonable min values: %v", bounds.Min)
	}

	if bounds.Dx() > 10000 || bounds.Dy() > 10000 {
		t.Errorf(
			"ActiveScreenBounds() returned unreasonably large dimensions: %dx%d",
			bounds.Dx(),
			bounds.Dy(),
		)
	}
}

func TestShowConfigValidationError(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		configPath   string
	}{
		{
			name:         "show error with valid message",
			errorMessage: "Invalid configuration",
			configPath:   "/path/to/config.yml",
		},
		{
			name:         "show error with empty message",
			errorMessage: "",
			configPath:   "/path/to/config.yml",
		},
		{
			name:         "show error with empty path",
			errorMessage: "Invalid configuration",
			configPath:   "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Initialize logger for testing
			bridge.InitializeLogger(zap.NewNop())

			// Note: This will actually show a dialog in test environment
			// In a real test environment, we'd mock the C function
			// For now, we just verify it doesn't panic
			// result := ShowConfigValidationError(tt.errorMessage, tt.configPath)

			// Skip actual execution in tests to avoid UI dialogs
			t.Skip(
				"Skipping UI dialog test - would require mocking C functions",
			)
		})
	}
}

func TestSetApplicationAttribute(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		attribute string
		value     bool
	}{
		{
			name:      "set attribute true",
			pid:       1234,
			attribute: "AXManualAccessibility",
			value:     true,
		},
		{
			name:      "set attribute false",
			pid:       1234,
			attribute: "AXManualAccessibility",
			value:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Initialize logger for testing
			bridge.InitializeLogger(zap.NewNop())

			// Note: This requires actual accessibility permissions
			// In a real test environment, we'd mock the C function
			// For now, we just verify it doesn't panic
			// result := SetApplicationAttribute(tt.pid, tt.attribute, tt.value)

			// Skip actual execution in tests to avoid permission requirements
			t.Skip("Skipping accessibility test - requires system permissions")
		})
	}
}
