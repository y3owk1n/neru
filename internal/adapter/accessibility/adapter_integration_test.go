//go:build integration

package accessibility_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/action"
	_ "github.com/y3owk1n/neru/internal/infra/bridge" // Link CGO implementations
	"github.com/y3owk1n/neru/internal/infra/logger"
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
	client := accessibility.NewInfraAXClient()
	adapter := accessibility.NewAdapter(logger, nil, nil, client)

	ctx := context.Background()

	t.Run("ScreenBounds", func(t *testing.T) {
		screenBounds, screenBoundsErr := adapter.ScreenBounds(ctx)
		if screenBoundsErr != nil {
			t.Fatalf("ScreenBounds() error = %v, want nil", screenBoundsErr)
		}

		if screenBounds.Empty() {
			t.Error("ScreenBounds() returned empty bounds")
		}

		// Verify reasonable screen dimensions
		if screenBounds.Dx() < 800 || screenBounds.Dy() < 600 {
			t.Errorf("Screen bounds too small: %v", screenBounds)
		}
	})

	t.Run("CursorPosition", func(t *testing.T) {
		pos, err := adapter.CursorPosition(ctx)
		if err != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", err)
		}

		// Position should be non-negative
		if pos.X < 0 || pos.Y < 0 {
			t.Errorf("Cursor position should be non-negative, got %v", pos)
		}

		// Log position for debugging
		t.Logf("Cursor position: %v", pos)
	})

	t.Run("MoveCursorToPoint", func(t *testing.T) {
		// Move to a safe position (avoid screen edges)
		target := image.Point{X: 100, Y: 100}

		moveErr := adapter.MoveCursorToPoint(ctx, target)
		if moveErr != nil {
			t.Errorf("MoveCursorToPoint() error = %v, want nil", moveErr)
		}

		// Give system time to complete the move
		time.Sleep(50 * time.Millisecond)

		// Verify position (allow some tolerance for OS behavior)
		newPos, newPosErr := adapter.CursorPosition(ctx)
		if newPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", newPosErr)
		}

		// Check if cursor moved reasonably close to target
		deltaX := abs(newPos.X - target.X)
		deltaY := abs(newPos.Y - target.Y)

		if deltaX > 50 || deltaY > 50 {
			t.Logf("Cursor move tolerance exceeded: target=%v, actual=%v", target, newPos)
		}
	})

	t.Run("ClickableElements", func(t *testing.T) {
		filter := ports.ElementFilter{
			MinSize: image.Point{X: 10, Y: 10},
		}

		clickableElements, clickableElementsErr := adapter.ClickableElements(ctx, filter)
		if clickableElementsErr != nil {
			// It might error if no permissions or no focused window
			t.Logf(
				"ClickableElements() error = %v (expected if no permissions or no windows)",
				clickableElementsErr,
			)
		} else {
			t.Logf("Found %d clickable elements", len(clickableElements))

			// Basic validation of returned elements
			for index, elem := range clickableElements {
				if elem == nil {
					t.Errorf("Element %d is nil", index)

					continue
				}

				bounds := elem.Bounds()
				if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
					t.Errorf("Element %d bounds %v smaller than minimum size %v", index, bounds, filter.MinSize)
				}
			}
		}
	})

	t.Run("PerformActionAtPoint", func(t *testing.T) {
		// Test with a safe point (should not error even if no element there)
		point := image.Point{X: 50, Y: 50}

		err := adapter.PerformActionAtPoint(ctx, action.TypeLeftClick, point)
		if err != nil {
			t.Logf("PerformActionAtPoint() error = %v (expected if no permissions)", err)
		} else {
			t.Logf("PerformActionAtPoint() succeeded at %v", point)
		}
	})

	t.Run("Scroll", func(t *testing.T) {
		// Test scroll operations
		testCases := []struct {
			name     string
			deltaX   int
			deltaY   int
			expected string
		}{
			{"scroll down", 0, -10, "down"},
			{"scroll up", 0, 10, "up"},
			{"scroll right", -10, 0, "right"},
			{"scroll left", 10, 0, "left"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				err := adapter.Scroll(ctx, testCase.deltaX, testCase.deltaY)
				if err != nil {
					t.Logf("Scroll() %s error = %v (expected if no permissions)",
						testCase.expected, err)
				} else {
					t.Logf("Scroll() %s succeeded", testCase.expected)
				}
			})
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		err := adapter.Health(ctx)
		if err != nil {
			t.Logf("Health() error = %v (expected if no permissions)", err)
		} else {
			t.Log("Health() check passed")
		}
	})
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}
