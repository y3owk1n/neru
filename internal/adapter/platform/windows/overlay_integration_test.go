//go:build integration && windows

package windows_test

import (
	"image"
	"strings"
	"sync"
	"testing"

	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

// Real Win32 overlay integration tests.
// Does not run in default CI; execute on WIN-VM desktop session with:
// go test -tags=integration ./internal/adapter/platform/windows/...
func skipIfOverlayUnavailable(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	msg := err.Error()
	if strings.Contains(msg, "interactive") ||
		strings.Contains(msg, "CreateWindowExW") ||
		strings.Contains(msg, "RegisterClassExW") {
		t.Skipf("skipping: overlay requires interactive desktop (%v)", err)
	}
}

func TestOverlayWindowLifecycleIntegration(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindow()
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindow: %v", err)
	}

	defer overlay.Destroy()

	if !overlay.Healthy() {
		t.Fatal("overlay is not healthy after creation")
	}

	bounds := overlay.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("overlay bounds = %v", bounds)
	}

	overlay.Clear()
	overlay.FillRect(bounds, 0x8000FF00)
	overlay.DrawTextCentered("FF", bounds, "Segoe UI", 48, 0xFFFFFFFF, true)
	_ = overlay.Flush()
	overlay.Show()
	overlay.Hide()

	err = overlay.ResizeToActiveScreen()
	if err != nil {
		t.Fatalf("ResizeToActiveScreen: %v", err)
	}
}

func TestOverlayRectDrawingIntegration(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindow()
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindow: %v", err)
	}

	defer overlay.Destroy()

	cell := image.Rect(10, 10, 110, 60)

	overlay.Clear()
	overlay.FillRect(cell, 0xCC3366FF)
	overlay.StrokeRect(cell, 0xFFFFFFFF, 2)
	_ = overlay.Flush()
}

// TestOverlayWindow_Show_RecreateDoesNotRaceCallerBoundsAccess recreates the
// window on the overlay UI thread while another goroutine resizes, reads and
// draws on it, as rebuildOnGDI does while the manager draws. Only -race
// reports a failure here. Whether a mid-create ResizeTo reaches the window
// is pinned by TestOverlayWindow_Show_RecreateCatchesUpToAMidCreateResize.
func TestOverlayWindow_Show_RecreateDoesNotRaceCallerBoundsAccess(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindowAt(0, 0, 64, 64)
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindowAt: %v", err)
	}

	defer overlay.Destroy()

	const iterations = 200

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		for range iterations {
			overlay.Destroy()
			overlay.Show()
		}
	})

	waitGroup.Go(func() {
		for i := range iterations {
			size := 32 + i%32
			_ = overlay.ResizeTo(i%16, i%16, size, size)
			_ = overlay.Bounds()
			overlay.FillRect(image.Rect(0, 0, size, size), 0x8000FF00)
		}
	})

	waitGroup.Wait()
}

// TestOverlayWindow_Show_PresentsAClearQueuedBeforeTheHide covers the mode
// switch in #1906. A mode exit queues a clear and hides without flushing it.
// The next mode's Show has to present that clear, and nothing else, before the
// window comes up. Skipping the clear left the previous mode's pixels on
// screen until a later flush. Repainting them under the clear presented them
// for one frame.
func TestOverlayWindow_Show_PresentsAClearQueuedBeforeTheHide(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindow()
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindow: %v", err)
	}

	defer overlay.Destroy()

	var (
		framesMu sync.Mutex
		frames   []winplatform.FrameStats
	)

	overlay.SetFrameObserver(func(stats winplatform.FrameStats) {
		framesMu.Lock()
		defer framesMu.Unlock()

		frames = append(frames, stats)
	})

	cell := image.Rect(10, 10, 110, 60)

	overlay.FillRect(cell, 0xCC3366FF)
	_ = overlay.Flush()
	overlay.Show()

	overlay.Clear()
	overlay.Hide()

	framesMu.Lock()
	frames = nil
	framesMu.Unlock()

	overlay.Show()

	framesMu.Lock()
	defer framesMu.Unlock()

	if len(frames) != 1 {
		t.Fatalf("Show presented %d frames, want only the queued clear", len(frames))
	}

	cleared := frames[0]
	if cleared.Err != nil || cleared.Commands != 0 || !cleared.Dirty.Overlaps(cell) {
		t.Fatalf("Show presented %+v, want a clear over %v", cleared, cell)
	}
}
