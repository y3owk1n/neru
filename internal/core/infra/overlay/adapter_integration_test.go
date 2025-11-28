//go:build integration

package overlay_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/overlay"
	"github.com/y3owk1n/neru/internal/core/ports"
	overlayManager "github.com/y3owk1n/neru/internal/ui/overlay"
)

// TestOverlayAdapterImplementsPort verifies the adapter implements the port interface.
func TestOverlayAdapterImplementsPort(_ *testing.T) {
	var _ ports.OverlayPort = (*overlay.Adapter)(nil)
}

// TestOverlayAdapterIntegration tests the overlay adapter with real dependencies.
func TestOverlayAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	logger := logger.Get()
	manager := overlayManager.Init(logger)
	adapter := overlay.NewAdapter(manager, logger)

	ctx := context.Background()

	t.Run("ShowHints", func(t *testing.T) {
		// ShowHints should not error with empty hints
		showHintsErr := adapter.ShowHints(ctx, nil)
		if showHintsErr != nil {
			t.Errorf("ShowHints() error = %v, want nil", showHintsErr)
		}
	})

	t.Run("ShowGrid", func(t *testing.T) {
		// ShowGrid should not error with valid dimensions
		showGridErr := adapter.ShowGrid(ctx)
		if showGridErr != nil {
			t.Errorf("ShowGrid() error = %v, want nil", showGridErr)
		}
	})

	t.Run("Hide", func(t *testing.T) {
		// Hide should not error
		hideErr := adapter.Hide(ctx)
		if hideErr != nil {
			t.Errorf("Hide() error = %v, want nil", hideErr)
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		// Refresh should not error
		refreshErr := adapter.Refresh(ctx)
		if refreshErr != nil {
			t.Errorf("Refresh() error = %v, want nil", refreshErr)
		}
	})

	t.Run("IsVisible", func(_ *testing.T) {
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

	logger := logger.Get()
	manager := overlayManager.Init(logger)
	adapter := overlay.NewAdapter(manager, logger)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("ShowHints with canceled context", func(t *testing.T) {
		showHintsErr := adapter.ShowHints(ctx, nil)
		if !errors.Is(showHintsErr, context.Canceled) {
			t.Errorf(
				"ShowHints() with canceled context error = %v, want %v",
				showHintsErr,
				context.Canceled,
			)
		}
	})

	t.Run("ShowGrid with canceled context", func(t *testing.T) {
		showGridErr := adapter.ShowGrid(ctx)
		if !errors.Is(showGridErr, context.Canceled) {
			t.Errorf(
				"ShowGrid() with canceled context error = %v, want %v",
				showGridErr,
				context.Canceled,
			)
		}
	})
}
