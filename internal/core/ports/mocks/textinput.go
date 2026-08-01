package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockTextInputPort is a mock implementation of ports.TextInputPort.
//
// By default it reports started == false, matching every platform but macOS —
// so a test exercising the fallback path (reading the event tap's key stream)
// gets that behavior without configuring anything. Set Started to true to
// exercise the native-field path instead.
type MockTextInputPort struct {
	// Started is what StartHintSearchSession reports. False by default.
	Started bool
	// StartErr and StopErr are returned by the respective calls when set.
	StartErr error
	StopErr  error

	mu        sync.Mutex
	callbacks ports.TextInputCallbacks
	frame     ports.TextInputFrame
	startN    int
	stopN     int
}

// StartHintSearchSession implements ports.TextInputPort.
func (m *MockTextInputPort) StartHintSearchSession(
	_ context.Context,
	callbacks ports.TextInputCallbacks,
	frame ports.TextInputFrame,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = callbacks
	m.frame = frame
	m.startN++

	return m.Started, m.StartErr
}

// StopHintSearchSession implements ports.TextInputPort.
func (m *MockTextInputPort) StopHintSearchSession(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopN++

	return m.StopErr
}

// Frame returns the frame passed to the last StartHintSearchSession call.
func (m *MockTextInputPort) Frame() ports.TextInputFrame {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.frame
}

// StartCount and StopCount report how many times each call was made.
func (m *MockTextInputPort) StartCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.startN
}

// StopCount reports how many times StopHintSearchSession was called.
func (m *MockTextInputPort) StopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopN
}

// EmitQueryChanged invokes the registered OnQueryChanged callback, standing in
// for the user typing into the native field.
func (m *MockTextInputPort) EmitQueryChanged(query string) {
	m.mu.Lock()
	callback := m.callbacks.OnQueryChanged
	m.mu.Unlock()

	if callback != nil {
		callback(query)
	}
}

// EmitConfirm invokes the registered OnConfirm callback.
func (m *MockTextInputPort) EmitConfirm() {
	m.mu.Lock()
	callback := m.callbacks.OnConfirm
	m.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// EmitCancel invokes the registered OnCancel callback.
func (m *MockTextInputPort) EmitCancel() {
	m.mu.Lock()
	callback := m.callbacks.OnCancel
	m.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// Ensure MockTextInputPort implements ports.TextInputPort.
var _ ports.TextInputPort = (*MockTextInputPort)(nil)
