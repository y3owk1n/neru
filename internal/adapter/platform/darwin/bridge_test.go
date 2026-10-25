//go:build darwin

package darwin_test

import (
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// MockAppWatcher implements AppWatcher for testing.
type MockAppWatcher struct {
	launchCalls        []AppEvent
	terminateCalls     []AppEvent
	activateCalls      []AppEvent
	deactivateCalls    []AppEvent
	screenChangeCalls  atomic.Int64
	mcActivatedCalls   atomic.Int64
	mcDeactivatedCalls atomic.Int64
}

// AppEvent is an app event.
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
	m.screenChangeCalls.Add(1)
}

func (m *MockAppWatcher) HandleMissionControlActivated() {
	m.mcActivatedCalls.Add(1)
}

func (m *MockAppWatcher) HandleMissionControlDeactivated() {
	m.mcDeactivatedCalls.Add(1)
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
			darwin.InitializeLogger(testCase.logger)

			if testCase.logger != nil && darwin.Logger() == nil {
				t.Error("Expected logger to be set")
			}
		})
	}
}

func TestSetAppWatcher(t *testing.T) {
	tests := []struct {
		name    string
		watcher darwin.AppWatcherInterface
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
			darwin.InitializeLogger(zap.NewNop())

			darwin.SetAppWatcher(testCase.watcher)

			if testCase.watcher != nil && darwin.AppWatcher() == nil {
				t.Error("Expected watcher to be set")
			}
		})
	}
}

func TestAppWatcherDispatchDroppedAfterClear(t *testing.T) {
	darwin.InitializeLogger(zap.NewNop())

	mock := &MockAppWatcher{}
	darwin.SetAppWatcher(mock)
	darwin.SetAppWatcher(nil)

	darwin.HandleAppLaunch("TestApp", "com.test.app")

	if len(mock.launchCalls) != 0 {
		t.Fatalf(
			"expected no dispatch after SetAppWatcher(nil), got %d calls",
			len(mock.launchCalls),
		)
	}
}

func TestCallbacks(t *testing.T) {
	darwin.InitializeLogger(zap.NewNop())

	mock := &MockAppWatcher{}
	darwin.SetAppWatcher(mock)
	t.Cleanup(func() {
		darwin.SetAppWatcher(nil)
	})

	// Enable Mission Control detection so the handlers forward events
	darwin.SetDetectMissionControlEnabled(true)
	t.Cleanup(func() {
		darwin.SetDetectMissionControlEnabled(false)
	})

	t.Run("HandleAppLaunch", func(t *testing.T) {
		darwin.HandleAppLaunch("TestApp", "com.test.app")

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
		darwin.HandleAppTerminate("TestApp", "com.test.app")

		if len(mock.terminateCalls) != 1 {
			t.Errorf("Expected 1 terminate call, got %d", len(mock.terminateCalls))
		}
	})

	t.Run("HandleAppActivate", func(t *testing.T) {
		darwin.HandleAppActivate("TestApp", "com.test.app")

		if len(mock.activateCalls) != 1 {
			t.Errorf("Expected 1 activate call, got %d", len(mock.activateCalls))
		}
	})

	t.Run("HandleAppDeactivate", func(t *testing.T) {
		darwin.HandleAppDeactivate("TestApp", "com.test.app")

		if len(mock.deactivateCalls) != 1 {
			t.Errorf("Expected 1 deactivate call, got %d", len(mock.deactivateCalls))
		}
	})

	t.Run("HandleScreenParametersChanged", func(t *testing.T) {
		darwin.HandleScreenParametersChanged()

		// The handler is dispatched in a goroutine, so wait briefly for it to complete.
		time.Sleep(50 * time.Millisecond)

		if got := mock.screenChangeCalls.Load(); got != 1 {
			t.Errorf("Expected 1 screen change call, got %d", got)
		}
	})

	t.Run("HandleMissionControlActivated", func(t *testing.T) {
		darwin.HandleMissionControlActivated()

		// The handler is dispatched in a goroutine, so wait briefly for it to complete.
		time.Sleep(50 * time.Millisecond)

		if got := mock.mcActivatedCalls.Load(); got != 1 {
			t.Errorf("Expected 1 mission control activated call, got %d", got)
		}
	})

	t.Run("HandleMissionControlDeactivated", func(t *testing.T) {
		darwin.HandleMissionControlDeactivated()

		// The handler is dispatched in a goroutine, so wait briefly for it to complete.
		time.Sleep(50 * time.Millisecond)

		if got := mock.mcDeactivatedCalls.Load(); got != 1 {
			t.Errorf("Expected 1 mission control deactivated call, got %d", got)
		}
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
			darwin.InitializeLogger(zap.NewNop())

			got := darwin.HasClickAction(testCase.element)
			if got != testCase.want {
				t.Errorf("HasClickAction() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestActiveScreenBounds(t *testing.T) {
	darwin.InitializeLogger(zap.NewNop())

	bounds := darwin.ActiveScreenBounds()

	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("ActiveScreenBounds() returned invalid bounds: %v", bounds)
	}

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

// TestSetApplicationAttribute pins the false a caller gets when the attribute
// cannot be set. Each row is one the bridge refuses whether or not this process
// holds accessibility permission, so the test never changes another app.
func TestSetApplicationAttribute(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		attribute string
	}{
		{
			name:      "unknown attribute on this process",
			pid:       os.Getpid(),
			attribute: "AXNeruNoSuchAttribute",
		},
		{
			name:      "real attribute on a pid no process holds",
			pid:       math.MaxInt32,
			attribute: "AXManualAccessibility",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			darwin.InitializeLogger(zap.NewNop())

			if darwin.SetApplicationAttribute(testCase.pid, testCase.attribute, true) {
				t.Errorf(
					"SetApplicationAttribute(%d, %q, true) = true, want false",
					testCase.pid,
					testCase.attribute,
				)
			}
		})
	}
}

// TestIsSecureInputEnabled exercises the CGO binding.
//
// The real value depends on whether a password field happens to be focused, so
// this cannot assert a specific result. It does assert the probe is a stable
// pure read. Two calls with nothing between them must agree, which catches an
// uninitialized bridge returning garbage.
func TestIsSecureInputEnabled(t *testing.T) {
	darwin.InitializeLogger(zap.NewNop())

	first := darwin.IsSecureInputEnabled()

	for range 5 {
		if darwin.IsSecureInputEnabled() != first {
			t.Fatal("IsSecureInputEnabled() returned different values on consecutive calls")
		}
	}
}
