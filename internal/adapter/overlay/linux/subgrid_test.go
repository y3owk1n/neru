//go:build linux && cgo

package linux

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
)

// subgridCancelWait is how long the cancel below is given before the test calls
// it absent. It is generous on purpose: the wait is only reached on a failure,
// so it costs a slow machine nothing and buys a named failure instead of a test
// binary that hangs until the package timeout.
const subgridCancelWait = 10 * time.Second

// TestLinuxOverlayManager_ShowSubgrid_BeginsTheFrameBeforeClearingIt pins the
// order the subgrid open runs the frame primitives in, the way
// TestLinuxOverlayManager_DrawGridPointer_BeginsTheFrameBeforeClearingIt pins it
// for the pointer's repaint of the same surface. beginFrame is what selects the
// writable buffer on Wayland, so clearing before it wipes the buffer that is on
// screen and leaves the one about to be shown holding the last frame — which on
// a wlroots compositor is a stale grid where the subgrid should be, on every
// subgrid open.
func TestLinuxOverlayManager_ShowSubgrid_BeginsTheFrameBeforeClearingIt(t *testing.T) {
	t.Parallel()

	overlayManager, surface := gridOnSurface(t)

	overlayManager.ShowSubgrid(firstGridCell(), gridcomponent.Style{})

	want := []string{"beginFrame", "surfaceClear", "surfaceFlush"}
	if !slices.Equal(surface.frameOps, want) {
		t.Errorf("frame ran %v, want %v", surface.frameOps, want)
	}
}

// TestLinuxOverlayManager_ShowSubgrid_CancelsTheAnimationWithRenderMuReleased
// pins the contract cancelAnimation states: the caller must not hold renderMu,
// because the goroutine being canceled takes that lock on every frame and would
// never reach the stop signal. Opening a subgrid while the surface is animating
// used to cancel from inside the lock, which is the deadlock that shape makes —
// the mode would hang with the grid still on screen.
//
// The stand-in below is the animation goroutine reduced to what makes the
// hazard: it is signaled to stop, and then needs renderMu to finish its frame.
// Asking for the lock rather than blocking on it is what lets this report the
// defect instead of hanging the test binary on it.
func TestLinuxOverlayManager_ShowSubgrid_CancelsTheAnimationWithRenderMuReleased(t *testing.T) {
	t.Parallel()

	overlayManager, _ := recordingManager()

	animStop := make(chan struct{})
	animDone := make(chan struct{})
	overlayManager.x11.animStop = animStop
	overlayManager.x11.animDone = animDone

	var lockWasFree atomic.Bool

	go func() {
		defer close(animDone)

		<-animStop

		if overlayManager.renderMu.TryLock() {
			lockWasFree.Store(true)
			overlayManager.renderMu.Unlock()
		}
	}()

	overlayManager.ShowSubgrid(firstGridCell(), gridcomponent.Style{})

	select {
	case <-animDone:
	case <-time.After(subgridCancelWait):
		t.Fatal("the animation was never canceled; it would paint over the subgrid")
	}

	if !lockWasFree.Load() {
		t.Error("the animation was canceled with renderMu held; " +
			"a goroutine that takes it on every frame never reaches the stop signal")
	}
}
