//go:build integration
// +build integration

package eventtap_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/eventtap"
	"github.com/y3owk1n/neru/internal/application/ports"
	_ "github.com/y3owk1n/neru/internal/infra/bridge" // Link CGO implementations
	eventtapInfra "github.com/y3owk1n/neru/internal/infra/eventtap"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestEventTapAdapterImplementsPort verifies the adapter implements the port interface.
func TestEventTapAdapterImplementsPort(t *testing.T) {
	var _ ports.EventTapPort = (*eventtap.Adapter)(nil)
}

// TestEventTapAdapterIntegration tests the event tap adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestEventTapAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()

	// Create infra event tap
	tap := eventtapInfra.NewEventTap(func(key string) {}, log)
	if tap == nil {
		t.Skip("Skipping EventTap test: failed to create event tap (missing permissions?)")
	}

	adapter := eventtap.NewAdapter(tap, log)

	ctx := context.Background()

	t.Run("Enable and Disable", func(t *testing.T) {
		// Enable
		err := adapter.Enable(ctx)
		if err != nil {
			t.Errorf("Enable() error = %v, want nil", err)
		}

		// Verify enabled
		if !adapter.IsEnabled() {
			t.Error("IsEnabled() = false, want true")
		}

		// Disable
		err = adapter.Disable(ctx)
		if err != nil {
			t.Errorf("Disable() error = %v, want nil", err)
		}

		// Verify disabled
		if adapter.IsEnabled() {
			t.Error("IsEnabled() = true, want false")
		}
	})

	t.Run("SetHandler", func(t *testing.T) {
		// SetHandler should not panic
		adapter.SetHandler(func(key string) {
			// Handler
		})
	})
}
