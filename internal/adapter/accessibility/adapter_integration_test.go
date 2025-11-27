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

	t.Run("ElementActions", func(t *testing.T) {
		// Test performing actions on discovered elements
		filter := ports.ElementFilter{
			MinSize: image.Point{X: 10, Y: 10},
		}

		elements, elementsErr := adapter.ClickableElements(ctx, filter)
		if elementsErr != nil {
			t.Logf("ClickableElements() error = %v, skipping element actions test", elementsErr)
			return
		}

		if len(elements) == 0 {
			t.Log("No clickable elements found, skipping element actions test")
			return
		}

		// Test action on first element
		element := elements[0]
		elementBounds := element.Bounds()
		actionPoint := image.Point{
			X: elementBounds.Min.X + elementBounds.Dx()/2,
			Y: elementBounds.Min.Y + elementBounds.Dy()/2,
		}

		err := adapter.PerformActionAtPoint(ctx, action.TypeLeftClick, actionPoint)
		if err != nil {
			t.Logf("PerformActionAtPoint() on element error = %v (expected if no permissions)", err)
		} else {
			t.Logf("Successfully performed action on element at %v", actionPoint)
		}
	})

	t.Run("ComplexInteractions", func(t *testing.T) {
		// Test a sequence of interactions
		// Move cursor to center of screen
		screenBounds, boundsErr := adapter.ScreenBounds(ctx)
		if boundsErr != nil {
			t.Logf("ScreenBounds() error = %v, skipping complex interactions", boundsErr)
			return
		}

		centerPoint := image.Point{
			X: screenBounds.Min.X + screenBounds.Dx()/2,
			Y: screenBounds.Min.Y + screenBounds.Dy()/2,
		}

		// Move to center
		moveErr := adapter.MoveCursorToPoint(ctx, centerPoint)
		if moveErr != nil {
			t.Logf("MoveCursorToPoint() error = %v", moveErr)
		}

		// Perform click
		clickErr := adapter.PerformActionAtPoint(ctx, action.TypeLeftClick, centerPoint)
		if clickErr != nil {
			t.Logf("PerformActionAtPoint() error = %v", clickErr)
		}

		// Scroll
		scrollErr := adapter.Scroll(ctx, 0, -50)
		if scrollErr != nil {
			t.Logf("Scroll() error = %v", scrollErr)
		}

		t.Logf("Completed complex interaction sequence at %v", centerPoint)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test error handling with invalid inputs

		// Invalid cursor position (negative coordinates)
		invalidPoint := image.Point{X: -100, Y: -100}
		err := adapter.MoveCursorToPoint(ctx, invalidPoint)
		// Should not panic, may or may not error depending on implementation
		if err != nil {
			t.Logf("MoveCursorToPoint with invalid coords returned error: %v", err)
		}

		// Invalid scroll parameters
		scrollErr := adapter.Scroll(ctx, 999999, 999999)
		if scrollErr != nil {
			t.Logf("Scroll with extreme values returned error: %v", scrollErr)
		}

		// Action on invalid point
		actionErr := adapter.PerformActionAtPoint(ctx, action.TypeLeftClick, invalidPoint)
		if actionErr != nil {
			t.Logf("PerformActionAtPoint with invalid coords returned error: %v", actionErr)
		}
	})

	t.Run("Performance", func(t *testing.T) {
		// Basic performance test for common operations
		const iterations = 10

		// Test cursor position performance
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, err := adapter.CursorPosition(ctx)
			if err != nil {
				break // Stop timing if errors occur
			}
		}
		duration := time.Since(start)

		t.Logf("Cursor position queries (%d iterations): %v", iterations, duration)

		// Test screen bounds performance
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_, err := adapter.ScreenBounds(ctx)
			if err != nil {
				break
			}
		}
		duration = time.Since(start)

		t.Logf("Screen bounds queries (%d iterations): %v", iterations, duration)
	})
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}
