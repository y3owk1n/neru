//go:build integration && windows

package windows

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// Real Win32 overlay integration tests for the recursive-grid depth
// transition. They need an interactive desktop session, which CI's
// windows-latest runner has:
// go test -tags=integration ./internal/adapter/overlay/windows/...
func newTestWinOverlay(t *testing.T) (*winOverlay, *sync.Mutex) {
	t.Helper()

	var renderMu sync.Mutex

	overlay := newWinOverlay(nil, &renderMu)
	if overlay == nil {
		t.Skip("skipping: overlay requires an interactive desktop")
	}

	if !overlay.Healthy() {
		overlay.Destroy()
		t.Skip("skipping: overlay window is not healthy")
	}

	t.Cleanup(func() {
		renderMu.Lock()
		overlay.Destroy()
		renderMu.Unlock()
	})

	return overlay, &renderMu
}

func drawDepth(
	overlay *winOverlay,
	renderMu *sync.Mutex,
	bounds image.Rectangle,
	depth int,
	enabled bool,
	duration time.Duration,
) chan struct{} {
	renderMu.Lock()
	defer renderMu.Unlock()

	dims := domain.GridDimensions{Cols: 2, Rows: 2}
	overlay.DrawRecursiveGrid(
		bounds, depth, "ABCD", dims, "ABCD", dims,
		recursivegridcomponent.Style{}, recursivegridcomponent.VirtualPointerState{},
		enabled, duration,
	)

	return overlay.transitionDone
}

func TestWinOverlayDrawRecursiveGrid_DepthChangeAnimatesForTheConfiguredDuration(t *testing.T) {
	overlay, renderMu := newTestWinOverlay(t)

	screen := image.Rect(0, 0, 400, 400)
	picked := image.Rect(200, 200, 400, 400)

	const duration = 80 * time.Millisecond

	if done := drawDepth(overlay, renderMu, screen, 1, true, duration); done != nil {
		t.Fatal("the first draw has no depth to zoom from and must paint in place")
	}

	started := time.Now()

	done := drawDepth(overlay, renderMu, picked, 2, true, duration)
	if done == nil {
		t.Fatal("a depth change with the animation enabled must start a transition")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the transition did not finish")
	}

	if elapsed := time.Since(started); elapsed < duration {
		t.Fatalf("the transition finished after %v, before its %v duration", elapsed, duration)
	}

	renderMu.Lock()
	defer renderMu.Unlock()

	want := recursivegrid.ComputeGridCells(picked, domain.GridDimensions{Cols: 2, Rows: 2})
	for idx, cell := range overlay.animRects {
		if cell != want[idx] {
			t.Fatalf("cell %d settled on %v, want %v", idx, cell, want[idx])
		}
	}
}

func TestWinOverlayDrawRecursiveGrid_ZeroDurationOrDisabledPaintsImmediately(t *testing.T) {
	overlay, renderMu := newTestWinOverlay(t)

	screen := image.Rect(0, 0, 400, 400)
	picked := image.Rect(200, 200, 400, 400)

	drawDepth(overlay, renderMu, screen, 1, true, 0)

	if done := drawDepth(overlay, renderMu, picked, 2, true, 0); done != nil {
		t.Fatal("a zero duration must paint the new depth immediately")
	}

	if done := drawDepth(overlay, renderMu, screen, 1, false, 80*time.Millisecond); done != nil {
		t.Fatal("recursive_grid.animation.enabled = false must paint the new depth immediately")
	}
}

func TestWinOverlayDrawRecursiveGrid_ANewDrawCancelsTheRunningTransition(t *testing.T) {
	overlay, renderMu := newTestWinOverlay(t)

	screen := image.Rect(0, 0, 400, 400)
	picked := image.Rect(200, 200, 400, 400)

	const duration = 2 * time.Second

	drawDepth(overlay, renderMu, screen, 1, true, duration)

	done := drawDepth(overlay, renderMu, picked, 2, true, duration)
	if done == nil {
		t.Fatal("a depth change with the animation enabled must start a transition")
	}

	// Another mode's draw takes the surface; the zoom must stop at once.
	renderMu.Lock()
	overlay.DrawHints(nil, hintscomponent.StyleMode{}, badge.HintOnTarget)
	renderMu.Unlock()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the transition kept running after another draw took the surface")
	}
}
