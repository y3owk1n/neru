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

// TestHotkeyAdapterIntegration tests the hotkey adapter.
// Note: This test requires a window manager environment and might fail in headless CI.
func TestHotkeyAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()
	adapter := hotkey.NewAdapter(log)

	ctx := context.Background()

	t.Run("Register and Unregister", func(t *testing.T) {
		// Use a complex hotkey that is unlikely to conflict
		key := "cmd+alt+ctrl+shift+f12"

		// Register
		err := adapter.Register(ctx, key, func() {
			// Callback
		})
		if err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}

		// Verify registered
		if !adapter.IsRegistered(ctx, key) {
			t.Error("IsRegistered() = false, want true")
		}

		// Unregister
		err = adapter.Unregister(ctx, key)
		if err != nil {
			t.Errorf("Unregister() error = %v, want nil", err)
		}

		// Verify unregistered
		if adapter.IsRegistered(ctx, key) {
			t.Error("IsRegistered() = true, want false")
		}
	})

	t.Run("Register Invalid Hotkey", func(t *testing.T) {
		err := adapter.Register(ctx, "invalid-hotkey", func() {})
		if err == nil {
			t.Error("Register() with invalid hotkey error = nil, want error")
		}
	})
}
