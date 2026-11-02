//go:build integration && windows

package windows

import (
	"errors"
	"image"
	"strings"
	"sync"
	"testing"
)

var errForcedGDI = errors.New("forced onto gdi by the test")

// TestOverlayWindow_Show_PresentsAClearQueuedBeforeTheHide covers the mode
// switch in #1906. A mode exit queues a clear and hides without flushing it.
// The next mode's Show has to present that clear, and nothing else, before the
// window comes up. Skipping the clear left the previous mode's pixels on
// screen until a later flush. Repainting them under the clear presented them
// for one frame. On GDI the clear lands on a surface the release resized, and
// it has to present the whole window or the old frame stays on it.
func TestOverlayWindow_Show_PresentsAClearQueuedBeforeTheHide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		forceGDI bool
	}{
		{name: "default surface"},
		{name: "gdi", forceGDI: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			overlay, err := NewOverlayWindow()
			if err != nil {
				msg := err.Error()
				if strings.Contains(msg, "interactive") ||
					strings.Contains(msg, "CreateWindowExW") ||
					strings.Contains(msg, "RegisterClassExW") {
					t.Skipf("skipping: overlay requires interactive desktop (%v)", err)
				}

				t.Fatalf("NewOverlayWindow: %v", err)
			}

			defer overlay.Destroy()

			if testCase.forceGDI {
				var rebuildErr error

				runOnOverlayUI(func() { rebuildErr = overlay.rebuildOnGDI(errForcedGDI) })

				if rebuildErr != nil {
					t.Fatalf("rebuildOnGDI: %v", rebuildErr)
				}

				if backend := overlay.Backend(); backend != "gdi" {
					t.Fatalf("Backend() = %q after the rebuild, want gdi", backend)
				}
			}

			var (
				framesMu sync.Mutex
				frames   []FrameStats
			)

			overlay.SetFrameObserver(func(stats FrameStats) {
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
		})
	}
}
