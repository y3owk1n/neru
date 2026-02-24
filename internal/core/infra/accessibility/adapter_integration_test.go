//go:build integration

package accessibility_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	_ "github.com/y3owk1n/neru/internal/core/infra/bridge" // Link CGO implementations
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// TestAccessibilityAdapterImplementsPort verifies the adapter implements the port interface.
func TestAccessibilityAdapterImplementsPort(_ *testing.T) {
	var _ ports.AccessibilityPort = (*accessibility.Adapter)(nil)
}

// TestAccessibilityAdapterIntegration tests the accessibility adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestAccessibilityAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()
	client := accessibility.NewInfraAXClient(logger, nil)
	adapter := accessibility.NewAdapter(logger, nil, nil, client, false)

	ctx := context.Background()

	t.Run("ScreenBounds", func(t *testing.T) {
		screenBounds, screenBoundsErr := adapter.ScreenBounds(ctx)
		if screenBoundsErr != nil {
			t.Fatalf("ScreenBounds() error = %v, want nil", screenBoundsErr)
		}

		if screenBounds.Empty() {
			t.Error("ScreenBounds() returned empty bounds")
		}
	})

	t.Run("CursorPosition", func(t *testing.T) {
		pos, err := adapter.CursorPosition(ctx)
		if err != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", err)
		}
		// Position should be within screen bounds (roughly)
		// We can't strictly enforce this as cursor might be on another screen
		_ = pos
	})

	t.Run("MoveCursorToPoint", func(t *testing.T) {
		// Get current position
		startPos, startPosErr := adapter.CursorPosition(ctx)
		if startPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", startPosErr)
		}

		// Move slightly
		target := image.Point{X: startPos.X + 10, Y: startPos.Y + 10}

		startPosErr = adapter.MoveCursorToPoint(ctx, target, false)
		if startPosErr != nil {
			t.Errorf("MoveCursorToPoint() error = %v, want nil", startPosErr)
		}

		// Verify position (might be slightly off due to OS acceleration/constraints)
		newPos, newPosErr := adapter.CursorPosition(ctx)
		if newPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", newPosErr)
		}

		// Just verify it moved or didn't error. Exact position check is flaky.
		_ = newPos
	})

	t.Run("MoveCursorToPoint bypassSmooth", func(t *testing.T) {
		// Get current position
		startPos, startPosErr := adapter.CursorPosition(ctx)
		if startPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", startPosErr)
		}

		// Move slightly with bypass smooth (direct movement)
		target := image.Point{X: startPos.X + 20, Y: startPos.Y + 20}

		startPosErr = adapter.MoveCursorToPoint(ctx, target, true)
		if startPosErr != nil {
			t.Errorf("MoveCursorToPoint(bypassSmooth=true) error = %v, want nil", startPosErr)
		}
	})

	t.Run("ClickableElements", func(t *testing.T) {
		// This is hard to test without a known window.
		// We'll just call it and ensure it doesn't panic or return error (unless permissions missing).
		filter := ports.ElementFilter{
			MinSize: image.Point{X: 10, Y: 10},
		}

		clickableElements, clickableElementsErr := adapter.ClickableElements(ctx, filter)
		if clickableElementsErr != nil {
			// It might error if no permissions or no focused window
			t.Logf(
				"ClickableElements() error = %v (expected if no permissions)",
				clickableElementsErr,
			)
		} else {
			t.Logf("Found %d elements", len(clickableElements))
		}
	})
}
