package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockEventTapPort is a mock implementation of ports.EventTapPort.
//
// It records the calls that mode setup and teardown make, so a test can assert
// on the sequence without a real keyboard grab.
type MockEventTapPort struct {
	// EnableFunc and DisableFunc override the default recording behavior.
	EnableFunc  func(context.Context) error
	DisableFunc func(context.Context) error

	// OnCall, when set, is invoked with a short label for every mutating
	// call. Tests that care about ordering use it to build a call log.
	OnCall func(label string)

	mu                      sync.Mutex
	enabled                 bool
	handler                 func(key string)
	hotkeys                 []string
	interceptedModifierKeys []string
	postedModifiers         []string
	passthroughEnabled      bool
	passthroughBlacklist    []string
	stickyModifierToggle    bool
	keyboardLayout          string
	destroyed               bool
}

// Enable implements ports.EventTapPort.
func (m *MockEventTapPort) Enable(ctx context.Context) error {
	m.mu.Lock()
	m.enabled = true
	m.mu.Unlock()

	m.record("enable")

	if m.EnableFunc != nil {
		return m.EnableFunc(ctx)
	}

	return nil
}

// Disable implements ports.EventTapPort.
func (m *MockEventTapPort) Disable(ctx context.Context) error {
	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()

	m.record("disable")

	if m.DisableFunc != nil {
		return m.DisableFunc(ctx)
	}

	return nil
}

// IsEnabled implements ports.EventTapPort.
func (m *MockEventTapPort) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.enabled
}

// SetHandler implements ports.EventTapPort.
func (m *MockEventTapPort) SetHandler(handler func(key string)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handler = handler
}

// SetHotkeys implements ports.EventTapPort.
func (m *MockEventTapPort) SetHotkeys(hotkeys []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hotkeys = append([]string(nil), hotkeys...)
}

// SetModifierPassthrough implements ports.EventTapPort.
func (m *MockEventTapPort) SetModifierPassthrough(enabled bool, blacklist []string) {
	m.mu.Lock()
	m.passthroughEnabled = enabled

	m.passthroughBlacklist = append([]string(nil), blacklist...)
	m.mu.Unlock()

	m.record("set_modifier_passthrough")
}

// SetInterceptedModifierKeys implements ports.EventTapPort.
func (m *MockEventTapPort) SetInterceptedModifierKeys(keys []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.interceptedModifierKeys = append([]string(nil), keys...)
}

// SetPassthroughCallback implements ports.EventTapPort.
func (m *MockEventTapPort) SetPassthroughCallback(_ func()) {}

// SetStickyModifierToggle implements ports.EventTapPort.
func (m *MockEventTapPort) SetStickyModifierToggle(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stickyModifierToggle = enabled
}

// SetKeyboardLayout implements ports.EventTapPort.
func (m *MockEventTapPort) SetKeyboardLayout(layoutID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.keyboardLayout = layoutID

	return true
}

// PostModifierEvent implements ports.EventTapPort.
func (m *MockEventTapPort) PostModifierEvent(modifier string, isDown bool) {
	m.mu.Lock()
	m.postedModifiers = append(m.postedModifiers, modifier)
	m.mu.Unlock()

	if isDown {
		m.record(modifier + "_down")

		return
	}

	m.record(modifier)
}

// Destroy implements ports.EventTapPort.
func (m *MockEventTapPort) Destroy() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.destroyed = true
}

// PostedModifiers returns the modifiers passed to PostModifierEvent, in order.
func (m *MockEventTapPort) PostedModifiers() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.postedModifiers...)
}

// Hotkeys returns the last hotkey set passed to SetHotkeys.
func (m *MockEventTapPort) Hotkeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.hotkeys...)
}

// Destroyed reports whether Destroy was called.
func (m *MockEventTapPort) Destroyed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.destroyed
}

// Ensure MockEventTapPort implements ports.EventTapPort.
var _ ports.EventTapPort = (*MockEventTapPort)(nil)

func (m *MockEventTapPort) record(label string) {
	if m.OnCall != nil {
		m.OnCall(label)
	}
}
