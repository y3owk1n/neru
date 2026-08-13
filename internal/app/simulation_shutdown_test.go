package app_test

// Shutdown journey. The daemon quitting is a user-visible path like any other,
// and the order it releases things in is what decides whether a key still in
// flight lands on something that still exists.

import (
	"testing"
)

// TestSimulation_ShutdownDrainsTheEventTapBeforeReleasingTheOverlay is the
// regression for #1515.
//
// Tearing the event tap down drains its key dispatcher, and that drain
// delivers whatever key was still queued into the mode handler, which draws.
// So the tap has to go first: released the other way round, a key arriving in
// the window between the two calls is handled after the overlay is gone, and
// on macOS the native window it would draw on has already been freed.
//
// The drain below is where the real taps spend their teardown, and pressing a
// mode hotkey from inside it is exactly the key the issue describes. What the
// journey asserts is the state that key found — an overlay still alive — rather
// than a frame reaching the screen, because in this path it would not reach
// one either way: the activation's own draw is refused by the already-canceled
// root context (`showFrame` keeps h.ctx for precisely that reason,
// `modes/frames.go`). What is not gated that way is everything else the port
// declares — mode teardown's ClearFrame deliberately uses a background context,
// and none of the grid surface's per-keystroke updates takes one at all — so
// the ordering is what covers them, and the overlay adapter's own guard
// (`TestAdapterDestroy_StopsEveryPortCallFromReachingTheBackend`) is what
// covers a caller that arrives anyway.
func TestSimulation_ShutdownDrainsTheEventTapBeforeReleasingTheOverlay(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	var (
		drained             bool
		overlayAliveAtDrain bool
	)

	sim.tap.drain = func() {
		drained = true
		overlayAliveAtDrain = !sim.overlay.isDestroyed()

		sim.press(gridHotkey)
	}

	sim.app.Cleanup()

	if !drained {
		t.Fatal("shutdown never tore the event tap down, so nothing was drained")
	}

	if !overlayAliveAtDrain {
		t.Error(
			"a key drained by the event tap's teardown was delivered after the overlay " +
				"had been destroyed",
		)
	}

	if !sim.overlay.isDestroyed() {
		t.Error("shutdown left the overlay unreleased")
	}
}
