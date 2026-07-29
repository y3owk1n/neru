//go:build integration && darwin

// This exercises the darwin platform package rather than the accessibility
// adapter, but it lives here on purpose: it drives the one global cursor, and so
// does TestAccessibilityAdapterIntegration. Tests in a single package run
// sequentially, whereas `go test ./...` runs packages concurrently, so splitting
// the two across packages makes them fight over the cursor and fail randomly.

package accessibility_test

import (
	"image"
	"testing"
	"time"

	darwinplatform "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// kCGEventMouseMoved, as passed through to the CGEvent bridge.
const eventMouseMoved = 5

// TestSmoothCursorSettlesOnTarget drives the smooth-cursor animator to a spread
// of absolute points and requires it to settle exactly on each one.
//
// This is the counterpart to the direct-move guard in
// TestAccessibilityAdapterIntegration, and it matters most while macOS
// Accessibility Zoom is engaged: the animator posts a move per step and each
// step first pans the zoom viewport, which itself drags the cursor. The two have
// to converge rather than fight, and the cursor has to end up somewhere the user
// can actually see.
func TestSmoothCursorSettlesOnTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	zoomed := darwinplatform.IsScreenZoomed()
	if zoomed {
		t.Log("Accessibility Zoom is engaged — exercising the zoomed path")
	}

	bounds := darwinplatform.ActiveScreenBounds()
	if bounds.Empty() {
		t.Skip("no active screen bounds")
	}

	startPos := darwinplatform.CursorPosition()
	defer darwinplatform.MoveMouse(startPos, true)

	width := bounds.Dx()
	height := bounds.Dy()
	targets := []image.Point{
		{X: bounds.Min.X + width/10, Y: bounds.Min.Y + height/10},
		{X: bounds.Min.X + width*9/10, Y: bounds.Min.Y + height*9/10},
		{X: bounds.Min.X + width/2, Y: bounds.Min.Y + height/2},
		{X: bounds.Min.X + width/10, Y: bounds.Min.Y + height*9/10},
	}

	const (
		tolerance     = 1
		settleTimeout = 3 * time.Second
	)

	for _, target := range targets {
		darwinplatform.MoveMouseSmooth(target, 20, eventMouseMoved)

		var landed image.Point

		deadline := time.Now().Add(settleTimeout)

		for {
			landed = darwinplatform.CursorPosition()

			offset := landed.Sub(target)
			if offset.X >= -tolerance && offset.X <= tolerance &&
				offset.Y >= -tolerance && offset.Y <= tolerance {
				break
			}

			if time.Now().After(deadline) {
				t.Errorf(
					"MoveMouseSmooth(%v) settled at %v, off by %v; zoomed=%t",
					target, landed, offset, zoomed,
				)

				break
			}

			time.Sleep(10 * time.Millisecond)
		}

		// While zoomed in, landing on the point is only half the job — the
		// viewport has to have followed, or the cursor is correct but off screen.
		if viewport, ok := darwinplatform.ZoomViewport(); ok && !landed.In(viewport) {
			t.Errorf(
				"MoveMouseSmooth(%v) landed at %v, outside the zoom viewport %v",
				target, landed, viewport,
			)
		}
	}
}

// TestDirectMoveOverridesInFlightAnimation requires a direct move to win against
// a smooth animation it interrupts.
//
// Canceling the animator closes its stop channel, but a worker that has already
// passed its cancellation check can still post one more step. That step lands
// after the direct move and drags the cursor back toward the abandoned target —
// and, while zoomed, drags the zoom viewport with it, which nothing afterwards
// corrects. The interruption is timing-dependent, so sweep the delay across a
// whole animation rather than trying a single one.
func TestDirectMoveOverridesInFlightAnimation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	bounds := darwinplatform.ActiveScreenBounds()
	if bounds.Empty() {
		t.Skip("no active screen bounds")
	}

	startPos := darwinplatform.CursorPosition()
	defer darwinplatform.MoveMouse(startPos, true)

	animationTarget := image.Point{
		X: bounds.Min.X + bounds.Dx()*9/10,
		Y: bounds.Min.Y + bounds.Dy()*9/10,
	}
	directTarget := image.Point{
		X: bounds.Min.X + bounds.Dx()/6,
		Y: bounds.Min.Y + bounds.Dy()/4,
	}

	const (
		maxInterruptDelayMs = 40
		repeats             = 2
	)

	for delayMs := range maxInterruptDelayMs + 1 {
		for range repeats {
			darwinplatform.MoveMouseSmooth(animationTarget, 20, eventMouseMoved)
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
			darwinplatform.MoveMouse(directTarget, true)

			time.Sleep(60 * time.Millisecond)

			landed := darwinplatform.CursorPosition()
			if landed != directTarget {
				t.Fatalf(
					"interrupting a smooth move after %dms left the cursor at %v, want %v (off by %v) — "+
						"a canceled animation step landed after the direct move",
					delayMs,
					landed,
					directTarget,
					landed.Sub(directTarget),
				)
			}
		}
	}
}
