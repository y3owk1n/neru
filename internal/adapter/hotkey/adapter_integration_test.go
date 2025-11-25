package hotkey_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/hotkey"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestHotkeyAdapterImplementsPort verifies the adapter implements the port interface.
func TestHotkeyAdapterImplementsPort(_ *testing.T) {
	var _ ports.HotkeyPort = (*hotkey.Adapter)(nil)
}

// MockHotkeyManager implements hotkey.InfraManager for testing.
type MockHotkeyManager struct {
	registered map[string]int
	nextID     int
}

func (m *MockHotkeyManager) Register(key string, _ func()) (int, error) {
	if key == "invalid-hotkey" {
		return 0, context.DeadlineExceeded // Simulate error
	}

	id := m.nextID
	m.nextID++
	m.registered[key] = id

	return id, nil
}

func (m *MockHotkeyManager) Unregister(id int) {
	for key, value := range m.registered {
		if value == id {
			delete(m.registered, key)

			break
		}
	}
}

func (m *MockHotkeyManager) UnregisterAll() {
	m.registered = make(map[string]int)
}

// TestHotkeyAdapterIntegration tests the hotkey adapter.
func TestHotkeyAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()
	mockManager := &MockHotkeyManager{
		registered: make(map[string]int),
		nextID:     1,
	}
	adapter := hotkey.NewAdapter(mockManager, logger)

	ctx := context.Background()

	t.Run("Register and Unregister", func(t *testing.T) {
		// Use a complex hotkey that is unlikely to conflict
		key := "cmd+alt+ctrl+shift+f12"

		// Register
		registerErr := adapter.Register(context, key, func() error {
			// Callback
			return nil
		})
		if registerErr != nil {
			t.Fatalf("Register() error = %v, want nil", registerErr)
		}

		// Verify registered
		if !adapter.IsRegistered(key) {
			t.Error("IsRegistered() = false, want true")
		}

		// Unregister
		unregisterErr := adapter.Unregister(context, key)
		if unregisterErr != nil {
			t.Errorf("Unregister() error = %v, want nil", unregisterErr)
		}

		// Verify unregistered
		if adapter.IsRegistered(key) {
			t.Error("IsRegistered() = true, want false")
		}
	})

	t.Run("Register Invalid Hotkey", func(t *testing.T) {
		err := adapter.Register(context, "invalid-hotkey", func() error { return nil })
		if err == nil {
			t.Error("Register() with invalid hotkey error = nil, want error")
		}
	})
}
