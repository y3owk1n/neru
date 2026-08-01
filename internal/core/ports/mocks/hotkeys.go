package mocks

import (
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockHotkeyPort is a mock implementation of ports.HotkeyPort.
//
// Registered callbacks are retained so a test can fire one with Trigger,
// standing in for the user pressing the hotkey.
type MockHotkeyPort struct {
	// RegisterFunc overrides Register entirely when set.
	RegisterFunc func(keyString string, callback ports.HotkeyCallback) (ports.HotkeyID, error)

	mu             sync.Mutex
	nextID         ports.HotkeyID
	callbacks      map[ports.HotkeyID]ports.HotkeyCallback
	keys           map[ports.HotkeyID]string
	unregisterAllN int
}

// Register implements ports.HotkeyPort.
func (m *MockHotkeyPort) Register(
	keyString string,
	callback ports.HotkeyCallback,
) (ports.HotkeyID, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(keyString, callback)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.callbacks == nil {
		m.callbacks = make(map[ports.HotkeyID]ports.HotkeyCallback)
		m.keys = make(map[ports.HotkeyID]string)
	}

	m.nextID++
	m.callbacks[m.nextID] = callback
	m.keys[m.nextID] = keyString

	return m.nextID, nil
}

// Unregister implements ports.HotkeyPort.
func (m *MockHotkeyPort) Unregister(hotkeyID ports.HotkeyID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.callbacks, hotkeyID)
	delete(m.keys, hotkeyID)
}

// UnregisterAll implements ports.HotkeyPort.
func (m *MockHotkeyPort) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.unregisterAllN++
	m.callbacks = nil
	m.keys = nil
}

// RegisteredKeys returns the key strings currently registered.
func (m *MockHotkeyPort) RegisteredKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := make([]string, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}

	return keys
}

// UnregisterAllCallCount returns how many times UnregisterAll was called.
func (m *MockHotkeyPort) UnregisterAllCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.unregisterAllN
}

// Trigger fires the callback registered under hotkeyID, if any.
func (m *MockHotkeyPort) Trigger(hotkeyID ports.HotkeyID) {
	m.mu.Lock()
	callback := m.callbacks[hotkeyID]
	m.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// Ensure MockHotkeyPort implements ports.HotkeyPort.
var _ ports.HotkeyPort = (*MockHotkeyPort)(nil)
