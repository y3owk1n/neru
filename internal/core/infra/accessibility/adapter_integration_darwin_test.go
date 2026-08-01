//go:build integration && darwin

package accessibility_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	darwinplatform "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// TestAccessibilityAdapterIntegration tests the accessibility adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestAccessibilityAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()
	client := accessibility.NewInfraAXClient(log, nil)

	adapter := accessibility.NewAdapter(log, nil, nil, client, false)
	system := darwinplatform.NewSystemAdapter()

	ctx := context.Background()

	t.Run("ScreenBounds", func(t *testing.T) {
		screenBounds, screenBoundsErr := system.ScreenBounds(ctx)
		if screenBoundsErr != nil {
			t.Fatalf("ScreenBounds() error = %v, want nil", screenBoundsErr)
		}

		if screenBounds.Empty() {
			t.Error("ScreenBounds() returned empty bounds")
		}
	})

	t.Run("CursorPosition", func(t *testing.T) {
		pos, err := system.CursorPosition(ctx)
		if err != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", err)
		}
		// Position should be within screen bounds (roughly)
		// We can't strictly enforce this as cursor might be on another screen
		_ = pos
	})

	t.Run("MoveCursorToPoint", func(t *testing.T) {
		// Get current position
		startPos, startPosErr := system.CursorPosition(ctx)
		if startPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", startPosErr)
		}

		// Move slightly
		target := image.Point{X: startPos.X + 10, Y: startPos.Y + 10}

		startPosErr = system.MoveCursorToPoint(ctx, target, false)
		if startPosErr != nil {
			t.Errorf("MoveCursorToPoint() error = %v, want nil", startPosErr)
		}

		// Verify position (might be slightly off due to OS acceleration/constraints)
		newPos, newPosErr := system.CursorPosition(ctx)
		if newPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", newPosErr)
		}

		// Just verify it moved or didn't error. Exact position check is flaky.
		_ = newPos
	})

	t.Run("MoveCursorToPoint bypassSmooth", func(t *testing.T) {
		// Get current position
		startPos, startPosErr := system.CursorPosition(ctx)
		if startPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", startPosErr)
		}

		// Move slightly with bypass smooth (direct movement)
		target := image.Point{X: startPos.X + 20, Y: startPos.Y + 20}

		startPosErr = system.MoveCursorToPoint(ctx, target, true)
		if startPosErr != nil {
			t.Errorf("MoveCursorToPoint(bypassSmooth=true) error = %v, want nil", startPosErr)
		}
	})

	t.Run("MoveCursorToPoint lands on the requested point", func(t *testing.T) {
		// Regression guard for the macOS Accessibility Zoom interaction: when
		// pointer-motion events are posted at the HID tap and the screen is
		// zoomed in, the window server rewrites their location to
		// zoomOrigin+(posted-displayCenter)/zoomFactor and the cursor lands
		// hundreds of points away. Posting at the session tap is exact in both
		// states, so this must hold whether or not zoom is engaged.
		if darwinplatform.IsScreenZoomed() {
			t.Log("Accessibility Zoom is engaged — exercising the zoomed path")
		}

		// Earlier subtests kick off a smooth-cursor animation. Let it settle,
		// then give the worker a moment to drain any step already in flight, so
		// a trailing animation frame cannot overwrite the positions asserted here.
		idleErr := system.WaitForCursorIdle(ctx)
		if idleErr != nil {
			t.Fatalf("WaitForCursorIdle() error = %v, want nil", idleErr)
		}

		time.Sleep(50 * time.Millisecond)

		screenBounds, screenBoundsErr := system.ScreenBounds(ctx)
		if screenBoundsErr != nil {
			t.Fatalf("ScreenBounds() error = %v, want nil", screenBoundsErr)
		}

		startPos, startPosErr := system.CursorPosition(ctx)
		if startPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", startPosErr)
		}

		defer func() {
			_ = system.MoveCursorToPoint(ctx, startPos, true)
		}()

		// Spread the targets across the display so that at least one of them
		// falls outside any plausible zoom viewport.
		width := screenBounds.Dx()
		height := screenBounds.Dy()
		targets := []image.Point{
			{X: screenBounds.Min.X + width/10, Y: screenBounds.Min.Y + height/10},
			{X: screenBounds.Min.X + width/2, Y: screenBounds.Min.Y + height/2},
			{X: screenBounds.Min.X + width*4/5, Y: screenBounds.Min.Y + height*4/5},
		}

		// One point of slack absorbs the float64->int rounding in the bridge.
		const tolerance = 1

		// The window server reflects a posted move in the cursor state
		// asynchronously, so poll rather than reading once. A cursor that is
		// merely late converges within a frame or two; one that is mispositioned
		// (the zoom bug lands it hundreds of points away) never converges.
		const settleTimeout = 500 * time.Millisecond

		for _, target := range targets {
			moveErr := system.MoveCursorToPoint(ctx, target, true)
			if moveErr != nil {
				t.Fatalf("MoveCursorToPoint(%v) error = %v, want nil", target, moveErr)
			}

			var (
				landed image.Point
				offset image.Point
			)

			deadline := time.Now().Add(settleTimeout)

			for {
				var landedErr error

				landed, landedErr = system.CursorPosition(ctx)
				if landedErr != nil {
					t.Fatalf("CursorPosition() error = %v, want nil", landedErr)
				}

				offset = landed.Sub(target)
				if offset.X >= -tolerance && offset.X <= tolerance &&
					offset.Y >= -tolerance && offset.Y <= tolerance {
					break
				}

				if time.Now().After(deadline) {
					t.Errorf(
						"MoveCursorToPoint(%v) settled at %v, off by %v; zoomed=%t",
						target, landed, offset, darwinplatform.IsScreenZoomed(),
					)

					break
				}

				time.Sleep(5 * time.Millisecond)
			}

			// Landing on the point is not enough while zoomed in: the viewport
			// has to have followed, or the cursor is correct but off screen.
			if viewport, zoomed := darwinplatform.ZoomViewportAt(
				landed,
			); zoomed &&
				!landed.In(viewport) {
				t.Errorf(
					"MoveCursorToPoint(%v) landed at %v, outside the zoom viewport %v",
					target, landed, viewport,
				)
			}
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
