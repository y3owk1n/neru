//go:build integration

package overlay_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/logger"
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
		showGridErr := adapter.ShowGrid(ctx, 3, 3)
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

	t.Run("ModeTransitions", func(t *testing.T) {
		// Test transitioning between different overlay modes

		// Start with hints
		hintsErr := adapter.ShowHints(ctx, nil)
		if hintsErr != nil {
			t.Errorf("ShowHints() error = %v", hintsErr)
		}

		// Check visibility
		if !adapter.IsVisible() {
			t.Error("Expected overlay to be visible after ShowHints")
		}

		// Switch to grid
		gridErr := adapter.ShowGrid(ctx, 5, 5)
		if gridErr != nil {
			t.Errorf("ShowGrid() error = %v", gridErr)
		}

		// Should still be visible
		if !adapter.IsVisible() {
			t.Error("Expected overlay to remain visible after mode switch")
		}

		// Refresh
		refreshErr := adapter.Refresh(ctx)
		if refreshErr != nil {
			t.Errorf("Refresh() error = %v", refreshErr)
		}

		// Hide
		hideErr := adapter.Hide(ctx)
		if hideErr != nil {
			t.Errorf("Hide() error = %v", hideErr)
		}

		// Should not be visible
		if adapter.IsVisible() {
			t.Error("Expected overlay to be hidden after Hide()")
		}
	})

	t.Run("MultipleGridSizes", func(t *testing.T) {
		// Test different grid configurations
		testCases := []struct {
			name  string
			cols  int
			rows  int
			valid bool
		}{
			{"small grid", 2, 2, true},
			{"medium grid", 5, 5, true},
			{"large grid", 10, 10, true},
			{"zero size", 0, 5, false},
			{"negative size", -1, 5, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := adapter.ShowGrid(ctx, tc.cols, tc.rows)

				if tc.valid && err != nil {
					t.Errorf("ShowGrid(%d, %d) error = %v, expected nil", tc.cols, tc.rows, err)
				} else if !tc.valid && err == nil {
					t.Errorf("ShowGrid(%d, %d) error = nil, expected error for invalid size", tc.cols, tc.rows)
				}

				// Clean up
				adapter.Hide(ctx)
			})
		}
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
		showGridErr := adapter.ShowGrid(ctx, 3, 3)
		if !errors.Is(showGridErr, context.Canceled) {
			t.Errorf(
				"ShowGrid() with canceled context error = %v, want %v",
				showGridErr,
				context.Canceled,
			)
		}
	})
}
