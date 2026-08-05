//go:build integration && darwin

package accessibility_test

import (
	"image"
	"sync"
	"testing"
	"time"

	darwinplatform "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// This exercises the darwin platform package rather than the accessibility
// adapter, but it lives here on purpose: it drives the one global cursor, and so
// does TestAccessibilityAdapterIntegration. Tests in a single package run
// sequentially, whereas `go test ./...` runs packages concurrently, so splitting
// the two across packages makes them fight over the cursor and fail randomly.
//
// kCGEventMouseMoved, as passed through to the CGEvent bridge.
const (
	eventMouseMoved = 5
	// buttonLeft is kCGMouseButtonLeft; plain moves ignore the button.
	buttonLeft = 0
)

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

	requireDesktop(t)
	requireInputPermission(t)

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
		darwinplatform.MoveMouseSmooth(target, 20, eventMouseMoved, buttonLeft)

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
		if viewport, ok := darwinplatform.ZoomViewportAt(landed); ok && !landed.In(viewport) {
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

	requireDesktop(t)
	requireInputPermission(t)

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
			darwinplatform.MoveMouseSmooth(animationTarget, 20, eventMouseMoved, buttonLeft)
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

// TestConcurrentMovesKeepCursorInViewport runs cursor moves from several
// goroutines at once and requires the cursor to end up somewhere visible.
//
// Cursor moves are not serialized across callers: every hotkey press dispatches
// its own goroutine, repeat-while-held runs another, and IPC actions run on the
// IPC goroutine. While zoomed, each move both pans the magnified region and
// posts the cursor, and if two moves interleave those halves the cursor is left
// at a correct coordinate outside the magnified region — a state nothing
// corrects until the next move. The final position is naturally undefined when
// callers race, but it must always be somewhere the user can see.
func TestConcurrentMovesKeepCursorInViewport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	requireDesktop(t)

	bounds := darwinplatform.ActiveScreenBounds()
	if bounds.Empty() {
		t.Skip("no active screen bounds")
	}

	if _, zoomed := darwinplatform.ZoomViewportAt(darwinplatform.CursorPosition()); !zoomed {
		t.Log("Accessibility Zoom is not engaged — this only checks for races, not visibility")
	}

	startPos := darwinplatform.CursorPosition()
	defer darwinplatform.MoveMouse(startPos, true)

	// Far apart, so an interleaved pan and post cannot accidentally agree.
	targets := []image.Point{
		{X: bounds.Min.X + bounds.Dx()/10, Y: bounds.Min.Y + bounds.Dy()/10},
		{X: bounds.Min.X + bounds.Dx()*9/10, Y: bounds.Min.Y + bounds.Dy()*9/10},
		{X: bounds.Min.X + bounds.Dx()*9/10, Y: bounds.Min.Y + bounds.Dy()/10},
		{X: bounds.Min.X + bounds.Dx()/10, Y: bounds.Min.Y + bounds.Dy()*9/10},
	}

	const (
		rounds        = 40
		movesPerRound = 6
	)

	for round := range rounds {
		var movers sync.WaitGroup

		for _, target := range targets {
			movers.Add(1)

			go func(destination image.Point) {
				defer movers.Done()

				for range movesPerRound {
					darwinplatform.MoveMouse(destination, true)
				}
			}(target)
		}

		movers.Wait()
		time.Sleep(80 * time.Millisecond)

		landed := darwinplatform.CursorPosition()

		viewport, zoomed := darwinplatform.ZoomViewportAt(landed)
		if !zoomed {
			continue
		}

		if !landed.In(viewport) {
			t.Fatalf(
				"round %d: concurrent moves left the cursor at %v, outside the magnified region %v — "+
					"a pan and a post from different moves interleaved",
				round,
				landed,
				viewport,
			)
		}
	}
}
