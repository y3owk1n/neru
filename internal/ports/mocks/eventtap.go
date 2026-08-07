package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
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
	//
	// Set it at construction, or through SetOnCall once anything else can
	// reach the mock: the port is called from background goroutines, so a bare
	// assignment races the read in record.
	OnCall func(label string)

	mu                      sync.Mutex
	enabled                 bool
	handler                 func(key string)
	hotkeys                 []string
	interceptedModifierKeys []string
	postedModifiers         []string
	passthroughEnabled      bool
	passthroughBlacklist    []string
	passthroughCallback     func()
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
func (m *MockEventTapPort) SetPassthroughCallback(cb func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.passthroughCallback = cb
}

// PassthroughCallback returns the currently registered passthrough callback,
// or nil. Tests use it to capture a callback before a mode change replaces it.
func (m *MockEventTapPort) PassthroughCallback() func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.passthroughCallback
}

// TriggerPassthrough fires the registered passthrough callback, standing in
// for a modifier shortcut passing through to the OS. No-op when none is set.
func (m *MockEventTapPort) TriggerPassthrough() {
	callback := m.PassthroughCallback()
	if callback != nil {
		callback()
	}
}

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

// ModifierPassthrough returns the last state passed to
// SetModifierPassthrough: whether unbound modifier chords reach the focused
// application, and the chords excluded from that.
func (m *MockEventTapPort) ModifierPassthrough() (bool, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.passthroughEnabled, append([]string(nil), m.passthroughBlacklist...)
}

// InterceptedModifierKeys returns the last key set passed to
// SetInterceptedModifierKeys.
func (m *MockEventTapPort) InterceptedModifierKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.interceptedModifierKeys...)
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

// SetOnCall installs the call log hook after construction, for a test that
// starts recording partway through — say, once the calls a fixture made on the
// way to the state under test are no longer interesting.
func (m *MockEventTapPort) SetOnCall(onCall func(label string)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.OnCall = onCall
}

// record invokes the call log hook, if one is installed.
//
// The hook is read under the mutex and called outside it: every caller of this
// is already out of its own critical section, and a hook that reaches back into
// the mock would otherwise deadlock.
func (m *MockEventTapPort) record(label string) {
	m.mu.Lock()
	onCall := m.OnCall
	m.mu.Unlock()

	if onCall != nil {
		onCall(label)
	}
}
