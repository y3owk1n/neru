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
func TestEventTapAdapterImplementsPort(_ *testing.T) {
	var _ ports.EventTapPort = (*eventtap.Adapter)(nil)
}

// TestEventTapAdapterIntegration tests the event tap adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestEventTapAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create infra event tap
	tap := eventtapInfra.NewEventTap(func(_ string) {}, logger)
	if tap == nil {
		t.Skip("Skipping EventTap test: failed to create event tap (missing permissions?)")
	}

	adapter := eventtap.NewAdapter(tap, logger)

	context := context.Background()

	t.Run("Enable and Disable", func(t *testing.T) {
		// Enable
		enableErr := adapter.Enable(context)
		if enableErr != nil {
			t.Errorf("Enable() error = %v, want nil", enableErr)
		}

		// Verify enabled
		if !adapter.IsEnabled() {
			t.Error("IsEnabled() = false, want true")
		}

		// Disable
		disableErr := adapter.Disable(context)
		if disableErr != nil {
			t.Errorf("Disable() error = %v, want nil", disableErr)
		}

		// Verify disabled
		if adapter.IsEnabled() {
			t.Error("IsEnabled() = true, want false")
		}
	})

	t.Run("SetHandler", func(_ *testing.T) {
		// SetHandler should not panic
		adapter.SetHandler(func(_ string) {
			// Handler
		})
	})
}
