//go:build integration

package hotkey_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/hotkey"
	"github.com/y3owk1n/neru/internal/application/ports"
	hotkeyInfra "github.com/y3owk1n/neru/internal/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestHotkeyAdapterImplementsPort verifies the adapter implements the port interface.
func TestHotkeyAdapterImplementsPort(_ *testing.T) {
	var _ ports.HotkeyPort = (*hotkey.Adapter)(nil)
}

// TestHotkeyAdapterIntegration tests the hotkey adapter with real macOS APIs.
func TestHotkeyAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create real macOS hotkey manager
	manager := hotkeyInfra.NewManager(logger)
	// Cast to InfraManager interface
	var infraManager hotkey.InfraManager = manager
	adapter := hotkey.NewAdapter(infraManager, logger)

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
}
