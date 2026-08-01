package mocks

import (
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockAppWatcherPort is a mock implementation of ports.AppWatcherPort.
//
// Registered callbacks are retained so a test can fire them directly with
// EmitActivate and friends, standing in for a platform focus event.
type MockAppWatcherPort struct {
	mu sync.Mutex

	started     bool
	stopped     bool
	mcDetection bool

	activateCallbacks   []ports.AppEventCallback
	deactivateCallbacks []ports.AppEventCallback
	terminateCallbacks  []ports.AppEventCallback
	screenCallbacks     []func()
	mcActivated         []func()
	mcDeactivated       []func()
}

// Start implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.started = true
}

// Stop implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopped = true
}

// OnActivate implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnActivate(callback ports.AppEventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activateCallbacks = append(m.activateCallbacks, callback)
}

// OnDeactivate implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnDeactivate(callback ports.AppEventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deactivateCallbacks = append(m.deactivateCallbacks, callback)
}

// OnTerminate implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnTerminate(callback ports.AppEventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.terminateCallbacks = append(m.terminateCallbacks, callback)
}

// OnScreenParametersChanged implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnScreenParametersChanged(callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.screenCallbacks = append(m.screenCallbacks, callback)
}

// OnMissionControlActivated implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnMissionControlActivated(callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mcActivated = append(m.mcActivated, callback)
}

// OnMissionControlDeactivated implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) OnMissionControlDeactivated(callback func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mcDeactivated = append(m.mcDeactivated, callback)
}

// SetMCDetection implements ports.AppWatcherPort.
func (m *MockAppWatcherPort) SetMCDetection(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mcDetection = enabled
}

// Started reports whether Start was called.
func (m *MockAppWatcherPort) Started() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.started
}

// Stopped reports whether Stop was called.
func (m *MockAppWatcherPort) Stopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopped
}

// MCDetectionEnabled reports the last value passed to SetMCDetection.
func (m *MockAppWatcherPort) MCDetectionEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.mcDetection
}

// EmitActivate fires every registered activate callback.
func (m *MockAppWatcherPort) EmitActivate(appName, bundleID string) {
	for _, callback := range m.snapshotAppCallbacks(func(m *MockAppWatcherPort) []ports.AppEventCallback {
		return m.activateCallbacks
	}) {
		callback(appName, bundleID)
	}
}

// EmitDeactivate fires every registered deactivate callback.
func (m *MockAppWatcherPort) EmitDeactivate(appName, bundleID string) {
	for _, callback := range m.snapshotAppCallbacks(func(m *MockAppWatcherPort) []ports.AppEventCallback {
		return m.deactivateCallbacks
	}) {
		callback(appName, bundleID)
	}
}

// EmitScreenParametersChanged fires every registered screen-change callback.
func (m *MockAppWatcherPort) EmitScreenParametersChanged() {
	m.mu.Lock()
	callbacks := append([]func(){}, m.screenCallbacks...)
	m.mu.Unlock()

	for _, callback := range callbacks {
		callback()
	}
}

func (m *MockAppWatcherPort) snapshotAppCallbacks(
	pick func(*MockAppWatcherPort) []ports.AppEventCallback,
) []ports.AppEventCallback {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]ports.AppEventCallback{}, pick(m)...)
}

// Ensure MockAppWatcherPort implements ports.AppWatcherPort.
var _ ports.AppWatcherPort = (*MockAppWatcherPort)(nil)
