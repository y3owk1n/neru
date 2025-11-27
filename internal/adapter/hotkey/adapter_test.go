package hotkey_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/hotkey"
	"go.uber.org/zap"
)

var errRegistrationFailed = errors.New("registration failed")

type mockInfraManager struct {
	registered map[string]int
	nextID     int
}

func (m *mockInfraManager) Register(key string, callback func()) (int, error) {
	if key == "error-key" {
		return 0, errRegistrationFailed
	}

	identifier := m.nextID
	m.nextID++

	if m.registered == nil {
		m.registered = make(map[string]int)
	}

	m.registered[key] = identifier

	return identifier, nil
}

func (m *mockInfraManager) Unregister(id int) {
	for key, value := range m.registered {
		if value == id {
			delete(m.registered, key)

			break
		}
	}
}

func (m *mockInfraManager) UnregisterAll() {
	m.registered = make(map[string]int)
}

func TestAdapter_NewAdapter(t *testing.T) {
	logger := zap.NewNop()
	mockManager := &mockInfraManager{}
	adapter := hotkey.NewAdapter(mockManager, logger)

	if adapter == nil {
		t.Fatal("NewAdapter() returned nil")
	}
}

func TestAdapter_Register(t *testing.T) {
	logger := zap.NewNop()
	mockManager := &mockInfraManager{}
	adapter := hotkey.NewAdapter(mockManager, logger)
	ctx := context.Background()

	t.Run("successful registration", func(t *testing.T) {
		err := adapter.Register(ctx, "test-key", func() error { return nil })
		if err != nil {
			t.Errorf("Register() error = %v, want nil", err)
		}

		if !adapter.IsRegistered("test-key") {
			t.Error("IsRegistered() = false, want true")
		}
	})

	t.Run("registration error", func(t *testing.T) {
		err := adapter.Register(ctx, "error-key", func() error { return nil })
		if err == nil {
			t.Error("Register() error = nil, want error")
		}
	})
}

func TestAdapter_Unregister(t *testing.T) {
	logger := zap.NewNop()
	mockManager := &mockInfraManager{}
	adapter := hotkey.NewAdapter(mockManager, logger)
	ctx := context.Background()

	// Register first
	err := adapter.Register(ctx, "test-key", func() error { return nil })
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Unregister
	err = adapter.Unregister(ctx, "test-key")
	if err != nil {
		t.Errorf("Unregister() error = %v, want nil", err)
	}

	if adapter.IsRegistered("test-key") {
		t.Error("IsRegistered() = true, want false")
	}

	// Unregister non-existent key should not error
	err = adapter.Unregister(ctx, "non-existent")
	if err != nil {
		t.Errorf("Unregister() error = %v, want nil", err)
	}
}

func TestAdapter_UnregisterAll(t *testing.T) {
	logger := zap.NewNop()
	mockManager := &mockInfraManager{}
	adapter := hotkey.NewAdapter(mockManager, logger)
	ctx := context.Background()

	// Register multiple
	err1 := adapter.Register(ctx, "key1", func() error { return nil })
	if err1 != nil {
		t.Fatalf("Register() error = %v", err1)
	}

	err2 := adapter.Register(ctx, "key2", func() error { return nil })
	if err2 != nil {
		t.Fatalf("Register() error = %v", err2)
	}

	// Unregister all
	err := adapter.UnregisterAll(ctx)
	if err != nil {
		t.Errorf("UnregisterAll() error = %v, want nil", err)
	}

	if adapter.IsRegistered("key1") || adapter.IsRegistered("key2") {
		t.Error("Keys should be unregistered")
	}
}

func TestAdapter_IsRegistered(t *testing.T) {
	logger := zap.NewNop()
	mockManager := &mockInfraManager{}
	adapter := hotkey.NewAdapter(mockManager, logger)

	if adapter.IsRegistered("non-existent") {
		t.Error("IsRegistered() = true for non-existent key, want false")
	}
}
