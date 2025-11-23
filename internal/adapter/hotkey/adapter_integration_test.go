//go:build integration

package hotkey_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/hotkey"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestHotkeyAdapterImplementsPort verifies the adapter implements the port interface.
func TestHotkeyAdapterImplementsPort(t *testing.T) {
	var _ ports.HotkeyPort = (*hotkey.Adapter)(nil)
}

// MockHotkeyManager implements hotkey.InfraManager for testing.
type MockHotkeyManager struct {
	registered map[string]int
	nextID     int
}

func (m *MockHotkeyManager) Register(key string, callback func()) (int, error) {
	if key == "invalid-hotkey" {
		return 0, context.DeadlineExceeded // Simulate error
	}
	id := m.nextID
	m.nextID++
	m.registered[key] = id
	return id, nil
}

func (m *MockHotkeyManager) Unregister(id int) {
	for k, v := range m.registered {
		if v == id {
			delete(m.registered, k)
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

	log := logger.Get()
	mockManager := &MockHotkeyManager{
		registered: make(map[string]int),
		nextID:     1,
	}
	adapter := hotkey.NewAdapter(mockManager, log)

	ctx := context.Background()

	t.Run("Register and Unregister", func(t *testing.T) {
		// Use a complex hotkey that is unlikely to conflict
		key := "cmd+alt+ctrl+shift+f12"

		// Register
		err := adapter.Register(ctx, key, func() error {
			// Callback
			return nil
		})
		if err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}

		// Verify registered
		if !adapter.IsRegistered(key) {
			t.Error("IsRegistered() = false, want true")
		}

		// Unregister
		err = adapter.Unregister(ctx, key)
		if err != nil {
			t.Errorf("Unregister() error = %v, want nil", err)
		}

		// Verify unregistered
		if adapter.IsRegistered(key) {
			t.Error("IsRegistered() = true, want false")
		}
	})

	t.Run("Register Invalid Hotkey", func(t *testing.T) {
		err := adapter.Register(ctx, "invalid-hotkey", func() error { return nil })
		if err == nil {
			t.Error("Register() with invalid hotkey error = nil, want error")
		}
	})
}
