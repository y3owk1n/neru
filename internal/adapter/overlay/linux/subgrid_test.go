//go:build linux && cgo

package linux

import (
	"image"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
)

// subgridCancelWait is how long the cancel below is given before the test calls
// it absent. It is generous on purpose: the wait is only reached on a failure,
// so it costs a slow machine nothing and buys a named failure instead of a test
// binary that hangs until the package timeout.
const subgridCancelWait = 10 * time.Second

// subgridTestKeys is a key per cell of the subgrid every overlay draws. The
// test below sets it on the backend directly because these managers are stood
// up without render components, and a manager with none leaves the keys it was
// last given alone (syncSublayerKeysLocked) — so an unset one draws a subgrid
// with no labels at all, which is a surface nothing can be asserted about.
const subgridTestKeys = "abcdefghi"

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

// TestLinuxOverlayManager_ShowSubgrid_IsWhatEveryRepaintAfterItDraws pins both
// halves of what #1491 asks for: that the open shows the subgrid *alone*, which
// is the macOS answer, and that every later repaint of the same surface shows
// exactly what the open left, because nothing between them is anything the user
// typed. The second half used to hold for the pointer's repaint and not for the
// narrowing one, which put the parent cells back underneath the subgrid on the
// first keystroke after an open.
//
// paintSubgridSurface (overlay_shared_cgo.go) carries the evidence for the first
// half. What makes it an assertion here rather than a claim is wantSubgridOnly:
// a spelt-out screen, so an open that also drew the parent cells fails even
// though every repaint after it agreed.
//
// Comparing the rectangles themselves rather than counting them is what extends
// the second half to geometry: a repaint that drew the subgrid somewhere else,
// or at another size, fails too.
func TestLinuxOverlayManager_ShowSubgrid_IsWhatEveryRepaintAfterItDraws(t *testing.T) {
	t.Parallel()

	// wantSubgridOnly is the whole of what an open subgrid puts on this surface:
	// one label per subgridTestKeys character, upper-cased as every grid label
	// is, and then the pointer stand-in that rides the same pass. A parent cell
	// label ("AAAA" and its fifteen siblings) appearing anywhere in this list is
	// the defect.
	wantSubgridOnly := []string{
		"A", "B", "C", "D", "E", "F", "G", "H", "I",
		gridPointerAppearance.Char,
	}

	// The pointer is put on the surface before the open so that it is part of
	// what the open paints, which leaves moving it below a repaint of the same
	// content rather than a change to it.
	repaints := []struct {
		name    string
		repaint func(*Manager)
	}{
		{
			name: "a narrowing keystroke",
			repaint: func(overlayManager *Manager) {
				overlayManager.UpdateGridMatches("a")
			},
		},
		{
			name: "the pointer moving",
			repaint: func(overlayManager *Manager) {
				overlayManager.DrawGridPointer(
					manager.ModeGrid,
					image.Pt(300, 300),
					gridPointerAppearance,
				)
			},
		},
	}

	for _, testCase := range repaints {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			overlayManager, surface := gridOnSurface(t)
			overlayManager.x11.sublayerKeys = subgridTestKeys

			overlayManager.DrawGridPointer(
				manager.ModeGrid,
				image.Pt(120, 240),
				gridPointerAppearance,
			)
			surface.forget()

			overlayManager.ShowSubgrid(firstGridCell(), gridcomponent.Style{})

			opened := surface.paintedStrings()
			openedRects := slices.Clone(surface.rects)

			if !slices.Equal(opened, wantSubgridOnly) {
				t.Fatalf("the open painted %v, want %v — the subgrid and nothing under it",
					opened, wantSubgridOnly)
			}

			surface.forget()

			testCase.repaint(overlayManager)

			if !slices.Equal(surface.paintedStrings(), opened) {
				t.Errorf("the repaint painted %v, want the %v the open left",
					surface.paintedStrings(), opened)
			}

			if !slices.Equal(surface.rects, openedRects) {
				t.Errorf("the repaint drew %d rectangles, want the %d the open left, unmoved",
					len(surface.rects), len(openedRects))
			}
		})
	}
}
