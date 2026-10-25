//go:build integration && windows

package windows_test

import (
	"image"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

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

var procGetWindowRect = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowRect")

// TestOverlayWindow_Show_RecreateRacingResizeToEndsAtItsBounds recreates the
// window on the overlay UI thread while another goroutine resizes, reads and
// draws on it, as rebuildOnGDI does while the manager draws. -race reports
// the unlocked accesses. The final check catches a ResizeTo that landed
// while the UI thread built the window and never reached it.
func TestOverlayWindow_Show_RecreateRacingResizeToEndsAtItsBounds(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindowAt(0, 0, 64, 64)
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindowAt: %v", err)
	}

	defer overlay.Destroy()

	const iterations = 200

	var (
		waitGroup sync.WaitGroup
		last      image.Rectangle
	)

	waitGroup.Go(func() {
		for range iterations {
			overlay.Destroy()
			overlay.Show()
		}
	})

	waitGroup.Go(func() {
		for i := range iterations {
			size := 32 + i%32
			last = image.Rect(i%16, i%16, i%16+size, i%16+size)
			_ = overlay.ResizeTo(last.Min.X, last.Min.Y, size, size)
			_ = overlay.Bounds()
			overlay.FillRect(image.Rect(0, 0, size, size), 0x8000FF00)
		}
	})

	waitGroup.Wait()

	hwnd := overlay.HWND()
	if hwnd == 0 {
		t.Fatal("overlay has no window after the last Show")
	}

	var rect windows.Rect

	ret, _, callErr := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		t.Fatalf("GetWindowRect: %v", callErr)
	}

	got := image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom))
	if got != last {
		t.Fatalf("window rect = %v, want the last ResizeTo %v", got, last)
	}
}
