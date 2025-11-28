package hotkey_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/hotkey"
	"go.uber.org/zap"
)

// mockInfraManager is a mock implementation of InfraManager for testing.
type mockInfraManager struct {
	registered map[string]int
}

func newMockInfraManager() *mockInfraManager {
	return &mockInfraManager{
		registered: make(map[string]int),
	}
}

func (m *mockInfraManager) Register(key string, callback func()) (int, error) {
	id := len(m.registered) + 1
	m.registered[key] = id

	return id, nil
}

func (m *mockInfraManager) Unregister(id int) {
	// Find and remove by id
	for key, registeredID := range m.registered {
		if registeredID == id {
			delete(m.registered, key)

			break
		}
	}
}

func (m *mockInfraManager) UnregisterAll() {
	m.registered = make(map[string]int)
}

func TestNewAdapter(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkey.NewAdapter(mockInfra, logger)

	if adapter == nil {
		t.Fatal("NewAdapter() returned nil")
	}
}

func TestAdapter_Register(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkey.NewAdapter(mockInfra, logger)
	ctx := context.Background()

	callback := func() error {
		return nil
	}

	// Register a hotkey
	err := adapter.Register(ctx, "Cmd+Shift+X", callback)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Check that it was registered in the mock
	if len(mockInfra.registered) != 1 {
		t.Errorf("Expected 1 registration in mock, got %d", len(mockInfra.registered))
	}

	if _, exists := mockInfra.registered["Cmd+Shift+X"]; !exists {
		t.Error("Hotkey not registered in mock")
	}
}

func TestAdapter_Unregister(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkey.NewAdapter(mockInfra, logger)
	ctx := context.Background()

	// Register first
	err := adapter.Register(ctx, "Cmd+X", func() error { return nil })
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify registration exists
	if len(mockInfra.registered) != 1 {
		t.Errorf("Expected 1 registration before Unregister, got %d", len(mockInfra.registered))
	}

	// Unregister and ensure registration is cleared in the mock
	_ = adapter.Unregister(ctx, "Cmd+X")

	if len(mockInfra.registered) != 0 {
		t.Errorf("Expected 0 registrations after Unregister, got %d", len(mockInfra.registered))
	}
}

func TestAdapter_UnregisterAll(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkey.NewAdapter(mockInfra, logger)
	ctx := context.Background()

	// Register multiple
	_ = adapter.Register(ctx, "Cmd+X", func() error { return nil })
	_ = adapter.Register(ctx, "Cmd+Y", func() error { return nil })

	// Verify registrations exist
	if len(mockInfra.registered) != 2 {
		t.Errorf("Expected 2 registrations before UnregisterAll, got %d", len(mockInfra.registered))
	}

	// Unregister all
	_ = adapter.UnregisterAll(ctx)

	// Ensure all registrations were removed in the mock
	if len(mockInfra.registered) != 0 {
		t.Errorf("Expected 0 registrations after UnregisterAll, got %d", len(mockInfra.registered))
	}
}
