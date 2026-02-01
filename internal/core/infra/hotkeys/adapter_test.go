package hotkeys_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"go.uber.org/zap"
)

// mockInfraManager is a mock implementation of InfraManager for testing.
type mockInfraManager struct {
	registered map[string]int
	callbacks  map[int]func()
	nextID     int
}

func newMockInfraManager() *mockInfraManager {
	return &mockInfraManager{
		registered: make(map[string]int),
		callbacks:  make(map[int]func()),
		nextID:     1,
	}
}

func (m *mockInfraManager) Register(key string, callback func()) (int, error) {
	id := m.nextID
	m.nextID++
	m.registered[key] = id
	m.callbacks[id] = callback

	return id, nil
}

func (m *mockInfraManager) Unregister(id int) {
	// Find and remove by id
	for key, registeredID := range m.registered {
		if registeredID == id {
			delete(m.registered, key)
			delete(m.callbacks, id)

			break
		}
	}
}

func (m *mockInfraManager) UnregisterAll() {
	m.registered = make(map[string]int)
	m.callbacks = make(map[int]func())
	m.nextID = 1
}

func TestNewAdapter(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkeys.NewAdapter(mockInfra, logger)

	if adapter == nil {
		t.Fatal("NewAdapter() returned nil")
	}
}

func TestAdapter_Register(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkeys.NewAdapter(mockInfra, logger)
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
	adapter := hotkeys.NewAdapter(mockInfra, logger)
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
	unregisterErr := adapter.Unregister(ctx, "Cmd+X")
	if unregisterErr != nil {
		t.Fatalf("Unregister() error = %v", unregisterErr)
	}

	if len(mockInfra.registered) != 0 {
		t.Errorf("Expected 0 registrations after Unregister, got %d", len(mockInfra.registered))
	}
}

func TestAdapter_UnregisterAll(t *testing.T) {
	logger := zap.NewNop()
	mockInfra := newMockInfraManager()
	adapter := hotkeys.NewAdapter(mockInfra, logger)
	ctx := context.Background()

	// Register multiple
	regErr1 := adapter.Register(ctx, "Cmd+X", func() error { return nil })
	if regErr1 != nil {
		t.Fatalf("Register() error = %v", regErr1)
	}

	regErr2 := adapter.Register(ctx, "Cmd+Y", func() error { return nil })
	if regErr2 != nil {
		t.Fatalf("Register() error = %v", regErr2)
	}

	// Verify registrations exist
	if len(mockInfra.registered) != 2 {
		t.Errorf("Expected 2 registrations before UnregisterAll, got %d", len(mockInfra.registered))
	}

	// Unregister all
	unregisterAllErr := adapter.UnregisterAll(ctx)
	if unregisterAllErr != nil {
		t.Fatalf("UnregisterAll() error = %v", unregisterAllErr)
	}

	// Ensure all registrations were removed in the mock
	if len(mockInfra.registered) != 0 {
		t.Errorf("Expected 0 registrations after UnregisterAll, got %d", len(mockInfra.registered))
	}
}
