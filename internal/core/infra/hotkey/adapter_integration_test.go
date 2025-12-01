//go:build integration

package hotkey_test

import (
	"context"
	"testing"

	_ "github.com/y3owk1n/neru/internal/core/infra/bridge" // Link CGO implementations
	"github.com/y3owk1n/neru/internal/core/infra/hotkey"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// TestHotkeyAdapterImplementsPort verifies the adapter implements the port interface.
func TestHotkeyAdapterImplementsPort(_ *testing.T) {
	var _ ports.HotkeyPort = (*hotkey.Adapter)(nil)
}

// RealHotkeyManagerWrapper wraps the real hotkeys.Manager to implement InfraManager interface.
type RealHotkeyManagerWrapper struct {
	manager *hotkeys.Manager
}

func (w *RealHotkeyManagerWrapper) Register(key string, callback func()) (int, error) {
	id, err := w.manager.Register(key, callback)

	return int(id), err
}

func (w *RealHotkeyManagerWrapper) Unregister(id int) {
	w.manager.Unregister(hotkeys.HotkeyID(id))
}

func (w *RealHotkeyManagerWrapper) UnregisterAll() {
	w.manager.UnregisterAll()
}

// TestHotkeyAdapterIntegration tests the hotkey adapter with real hotkey manager.
func TestHotkeyAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()
	realManager := hotkeys.NewManager(logger)

	hotkeys.SetGlobalManager(realManager) // Required for C callbacks
	defer hotkeys.SetGlobalManager(nil)

	manager := &RealHotkeyManagerWrapper{manager: realManager}
	adapter := hotkey.NewAdapter(manager, logger)

	ctx := context.Background()

	t.Run("Register and Unregister", func(t *testing.T) {
		// Use a complex hotkey that is unlikely to conflict
		key := "cmd+alt+ctrl+shift+f12"

		// Register
		registerErr := adapter.Register(ctx, key, func() error {
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
		unregisterErr := adapter.Unregister(ctx, key)
		if unregisterErr != nil {
			t.Errorf("Unregister() error = %v, want nil", unregisterErr)
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

	// Cleanup
	t.Cleanup(func() {
		_ = adapter.UnregisterAll(ctx)
	})
}
