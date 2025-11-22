//go:build integration
// +build integration

package overlay_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/logger"
	overlayManager "github.com/y3owk1n/neru/internal/ui/overlay"
)

// TestOverlayAdapterImplementsPort verifies the adapter implements the port interface.
func TestOverlayAdapterImplementsPort(t *testing.T) {
	var _ ports.OverlayPort = (*overlay.Adapter)(nil)
}

// TestOverlayAdapterIntegration tests the overlay adapter with real dependencies.
func TestOverlayAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	log := logger.Get()
	manager := overlayManager.Init(log)
	adapter := overlay.NewAdapter(manager, log)

	ctx := context.Background()

	t.Run("ShowHints", func(t *testing.T) {
		// ShowHints should not error with empty hints
		err := adapter.ShowHints(ctx, nil)
		if err != nil {
			t.Errorf("ShowHints() error = %v, want nil", err)
		}
	})

	t.Run("ShowGrid", func(t *testing.T) {
		// ShowGrid should not error with valid dimensions
		err := adapter.ShowGrid(ctx, 3, 3)
		if err != nil {
			t.Errorf("ShowGrid() error = %v, want nil", err)
		}
	})

	t.Run("Hide", func(t *testing.T) {
		// Hide should not error
		err := adapter.Hide(ctx)
		if err != nil {
			t.Errorf("Hide() error = %v, want nil", err)
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		// Refresh should not error
		err := adapter.Refresh(ctx)
		if err != nil {
			t.Errorf("Refresh() error = %v, want nil", err)
		}
	})

	t.Run("IsVisible", func(t *testing.T) {
		// IsVisible should return a boolean without error
		visible := adapter.IsVisible()
		_ = visible // Just verify it doesn't panic
	})
}

// TestOverlayAdapterContextCancellation tests context cancellation handling.
func TestOverlayAdapterContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()
	manager := overlayManager.Init(log)
	adapter := overlay.NewAdapter(manager, log)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("ShowHints with canceled context", func(t *testing.T) {
		err := adapter.ShowHints(ctx, nil)
		if err != context.Canceled {
			t.Errorf("ShowHints() with canceled context error = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("ShowGrid with canceled context", func(t *testing.T) {
		err := adapter.ShowGrid(ctx, 3, 3)
		if err != context.Canceled {
			t.Errorf("ShowGrid() with canceled context error = %v, want %v", err, context.Canceled)
		}
	})
}
