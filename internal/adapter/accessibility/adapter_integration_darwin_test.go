//go:build integration && darwin

package accessibility_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/native"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	darwinplatform "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestAccessibilityAdapterIntegration tests the accessibility adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestAccessibilityAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.Get()
	client := native.New(log, nil)

	adapter := accessibility.NewAdapter(log, nil, nil, client, false)
	system := darwinplatform.NewSystemAdapter()

	// Bounded rather than context.Background(), but note what this does and
	// does not buy: the AX client discards the context and calls straight into
	// the Objective-C bridge, so the deadline cannot interrupt a query that is
	// already wedged there. What it does do is stop the scan's per-source
	// goroutines from starting further work once it has expired.
	//
	// The actual hang guard is runWithinBudget, which watches from outside the
	// call. The budget is shared and sits far above the daemon's own per-scan
	// ceiling (modes.HintTimeout, 5s), so neither can trip on a merely slow run.
	ctx, cancel := context.WithTimeout(context.Background(), integrationScanBudget)
	defer cancel()

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

		// The cursor may legitimately sit on a secondary display, so the
		// active screen's bounds are not a valid box to test against. What is
		// always true under the shared coordinate contract (global top-left
		// origin, Y down, unscaled pixels) is that the position is finite and
		// plausible for a desktop. A backend that failed to read the position
		// and returned CGPointZero-as-error, or that leaked Cocoa's
		// bottom-left origin as a negative Y, fails here.
		const sane = 100000

		if pos.X < -sane || pos.X > sane || pos.Y < -sane || pos.Y > sane {
			t.Errorf("CursorPosition() = %v, outside any plausible desktop coordinate range", pos)
		}

		// Reading twice without moving must be stable; a jittering read means
		// the position is being derived from something other than HID state.
		again, err := system.CursorPosition(ctx)
		if err != nil {
			t.Fatalf("second CursorPosition() error = %v, want nil", err)
		}

		if again != pos {
			t.Errorf("CursorPosition() is unstable without a move: got %v then %v", pos, again)
		}
	})

	t.Run("MoveCursorToPoint", func(t *testing.T) {
		requireInputPermission(t)

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

		// This subtest uses the smooth-cursor path (bypassSmooth=false), so the
		// cursor animates toward the target over several frames and its exact
		// position at any instant is not assertable. What must hold is that it
		// ends up at the target: "MoveCursorToPoint lands on the requested
		// point" below pins exact placement for the direct path, and here we
		// only require that the animated move converges rather than silently
		// doing nothing.
		deadline := time.Now().Add(time.Second)

		newPos, newPosErr := system.CursorPosition(ctx)
		if newPosErr != nil {
			t.Fatalf("CursorPosition() error = %v, want nil", newPosErr)
		}

		for newPos != target && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)

			newPos, newPosErr = system.CursorPosition(ctx)
			if newPosErr != nil {
				t.Fatalf("CursorPosition() error = %v, want nil", newPosErr)
			}
		}

		if newPos != target {
			t.Errorf(
				"MoveCursorToPoint(%v) from %v settled at %v; the smooth move never reached its target",
				target,
				startPos,
				newPos,
			)
		}
	})

	t.Run("MoveCursorToPoint bypassSmooth", func(t *testing.T) {
		// Gated even though it only asserts the call returns no error: without
		// the permission that call succeeds while the cursor never moves, so a
		// pass here would say nothing about the path it exists to cover.
		requireInputPermission(t)

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
		requireInputPermission(t)

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

	t.Run("ClickableElements queries the live AX tree without error", func(t *testing.T) {
		requireInputPermission(t)

		// A `go test` binary is not a foreground app with its own window, so
		// the live AX tree usually yields zero elements here. That makes the
		// element *count* unassertable, and any per-element loop vacuous — the
		// filter semantics are pinned deterministically instead by
		// TestAdapter_ClickableElements_FilterContract, which drives the same
		// code through MockAXClient with known nodes.
		//
		// What only a live run can check is that the real Objective-C query
		// path completes against the real AX API and returns a well-formed
		// slice rather than erroring or handing back nil entries.
		const minSide = 10

		filter := ports.ElementFilter{
			MinSize:      image.Point{X: minSide, Y: minSide},
			ExcludeRoles: []element.Role{element.RoleWindow},
		}

		var (
			clickableElements []*element.Element
			err               error
		)

		// The scan is the one call here that can wedge, and the deadline on ctx
		// cannot stop it — see runWithinBudget for why.
		runWithinBudget(t, "ClickableElements", func() {
			clickableElements, err = adapter.ClickableElements(ctx, filter)
		})

		if err != nil {
			t.Fatalf("ClickableElements() error = %v, want nil", err)
		}

		seen := make(map[element.ID]int, len(clickableElements))

		for idx, found := range clickableElements {
			if found == nil {
				t.Fatalf("element %d is nil", idx)
			}

			if found.ID() == "" {
				t.Errorf("element %d has an empty ID", idx)
			}

			if prev, dup := seen[found.ID()]; dup {
				t.Errorf("elements %d and %d share ID %q", prev, idx, found.ID())
			}

			seen[found.ID()] = idx

			if bounds := found.Bounds(); bounds.Dx() < minSide || bounds.Dy() < minSide {
				t.Errorf(
					"element %d (%q) has bounds %v, smaller than the requested MinSize %v",
					idx, found.ID(), bounds, filter.MinSize,
				)
			}

			if found.Role() == element.RoleWindow {
				t.Errorf("element %d (%q) has an excluded role %q", idx, found.ID(), found.Role())
			}
		}

		t.Logf("live AX tree yielded %d clickable elements", len(clickableElements))
	})
}
