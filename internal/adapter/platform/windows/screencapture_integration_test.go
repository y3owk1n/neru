//go:build integration && windows

package windows

import (
	"context"
	"image"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real GDI capture tests. They need an interactive desktop: a session-0
// service, which is what a headless runner may be, has no desktop to read and
// the overlay refuses to open there, so every test here skips on that answer.
// Run with: go test -tags=integration ./internal/adapter/platform/windows/

// dpiAwarenessContextPerMonitorV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2.
const dpiAwarenessContextPerMonitorV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)-4

// captureTestDPIOnce makes the test process per-monitor-v2 DPI aware, which
// the shipped exe gets from its manifest and a go test binary does not. It is
// what makes the mixed-DPI assertion below mean something: an unaware process
// sees every monitor virtualized to 96 DPI, and the coordinates then agree by
// construction. Failing is fine, it means the context was already set.
var captureTestDPIOnce = sync.OnceFunc(func() {
	proc := user32.NewProc("SetProcessDpiAwarenessContext")
	if proc.Find() != nil {
		return
	}

	discardCall(proc.Call(dpiAwarenessContextPerMonitorV2))
})

// paintedOverlay opens the overlay, paints one opaque rectangle and presents
// it, skipping when there is no desktop to paint on.
func paintedOverlay(t *testing.T, rect image.Rectangle, argb uint32) *OverlayWindow {
	t.Helper()
	captureTestDPIOnce()

	overlay, err := NewOverlayWindow()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "interactive") ||
			strings.Contains(msg, "CreateWindowExW") ||
			strings.Contains(msg, "RegisterClassExW") {
			t.Skipf("skipping: capture needs an interactive desktop (%v)", err)
		}

		t.Fatalf("NewOverlayWindow: %v", err)
	}

	t.Cleanup(overlay.Destroy)

	overlay.Show()
	overlay.Clear()
	overlay.FillRect(rect, argb)

	flushErr := overlay.Flush()
	if flushErr != nil {
		t.Fatalf("Flush: %v", flushErr)
	}

	// UpdateLayeredWindow hands the frame to the compositor; give it a vblank
	// or two before reading it back.
	time.Sleep(100 * time.Millisecond)

	return overlay
}

// TestCaptureScreenRegion_ReadsBackAKnownColor is the acceptance check: a
// rectangle painted one color reads back as that color at the expected
// pixel, in the RGBA layout image.RGBA promises.
func TestCaptureScreenRegion_ReadsBackAKnownColor(t *testing.T) {
	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)
	paintedOverlay(t, painted, 0xFF10C040)

	img, err := CaptureScreenRegion(context.Background(), painted)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%v): %v", painted, err)
	}

	if img.Rect != image.Rect(0, 0, painted.Dx(), painted.Dy()) {
		t.Fatalf("captured %v for a %v region, want bounds starting at (0, 0)", img.Rect, painted)
	}

	if img.Stride != img.Rect.Dx()*bytesPerPixel {
		t.Errorf(
			"stride is %d for a %d-wide image; expected tightly packed",
			img.Stride,
			img.Rect.Dx(),
		)
	}

	r, g, b, a := img.At(painted.Dx()/2, painted.Dy()/2).RGBA()
	got := [4]uint32{r >> 8, g >> 8, b >> 8, a >> 8}

	want := [4]uint32{0x10, 0xC0, 0x40, 0xFF}
	for channel := range want {
		if got[channel] != want[channel] {
			t.Fatalf("center pixel is %v, want %v (the overlay painted 0xFF10C040)", got, want)
		}
	}
}

// TestCaptureScreenRegion_MatchesEnumDisplayMonitors pins the DPI contract: on
// a mixed-DPI arrangement, capturing each monitor's reported bounds yields a
// frame of exactly those dimensions. Under per-monitor-v2 both answers are
// physical pixels; a process that lost that awareness would read a scaled
// monitor back at the wrong size, and every hint on it would land off target.
func TestCaptureScreenRegion_MatchesEnumDisplayMonitors(t *testing.T) {
	captureTestDPIOnce()

	monitors, err := enumerateMonitors()
	if err != nil {
		t.Skipf("no monitors: %v", err)
	}

	for _, monitor := range monitors {
		img, captureErr := CaptureScreenRegion(context.Background(), monitor.bounds)
		if captureErr != nil {
			if strings.Contains(captureErr.Error(), "interactive desktop") {
				t.Skipf("skipping: %v", captureErr)
			}

			t.Fatalf("capturing monitor %v: %v", monitor.bounds, captureErr)
		}

		if img.Rect.Dx() != monitor.bounds.Dx() || img.Rect.Dy() != monitor.bounds.Dy() {
			t.Errorf(
				"monitor %v captured as %v; the frame must match the bounds EnumDisplayMonitors reports",
				monitor.bounds,
				img.Rect,
			)
		}
	}
}

// TestCaptureScreenRegion_ZeroRegionIsTheActiveScreen pins what the zero
// rectangle means: the monitor under the cursor, whole.
func TestCaptureScreenRegion_ZeroRegionIsTheActiveScreen(t *testing.T) {
	captureTestDPIOnce()

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	img, err := CaptureScreenRegion(context.Background(), image.Rectangle{})
	if err != nil {
		if strings.Contains(err.Error(), "interactive desktop") {
			t.Skipf("skipping: %v", err)
		}

		t.Fatalf("CaptureScreenRegion(zero): %v", err)
	}

	if img.Rect.Dx() != screen.Dx() || img.Rect.Dy() != screen.Dy() {
		t.Errorf("zero region captured %v, want the active screen %v", img.Rect, screen)
	}
}

// TestCaptureScreenRegion_RejectsARegionOffTheScreen pins the exact-region
// contract: a rectangle that leaves every monitor fails rather than coming
// back clipped, and a degenerate one is refused rather than widened.
func TestCaptureScreenRegion_RejectsARegionOffTheScreen(t *testing.T) {
	tests := []struct {
		name   string
		region image.Rectangle
	}{
		{name: "far off screen", region: image.Rect(1_000_000, 1_000_000, 1_000_100, 1_000_100)},
		{name: "degenerate", region: image.Rect(10, 10, 10, 50)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			img, err := CaptureScreenRegion(context.Background(), testCase.region)
			if err == nil {
				t.Fatalf("capturing %v succeeded, returning %v", testCase.region, img.Rect)
			}

			if img != nil {
				t.Error("a refused capture returned an image alongside its error")
			}
		})
	}
}

// TestClipToOwnMonitor_KeepsAWindowInsideItsMonitor pins the clip the focused
// window gets: the bounds a capture strategy is handed never leave the monitor
// the window is on. It uses the desktop window, which every session has and
// which spans the primary monitor exactly.
func TestClipToOwnMonitor_KeepsAWindowInsideItsMonitor(t *testing.T) {
	captureTestDPIOnce()

	desktop := windows.GetDesktopWindow()
	if desktop == 0 {
		t.Skip("no desktop window")
	}

	var rect windows.Rect

	ret, _, _ := procGetWindowRect.Call(uintptr(desktop), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		t.Skip("GetWindowRect on the desktop window failed")
	}

	frame := rectToImage(rect)
	overhanging := frame.Inset(-8)

	clipped, found, err := clipToOwnMonitor(desktop, overhanging)
	if err != nil {
		t.Fatalf("clipToOwnMonitor: %v", err)
	}

	if !found {
		t.Fatal("clipToOwnMonitor reported no window for the desktop")
	}

	if !clipped.In(frame) {
		t.Errorf("clipped %v to %v, which still leaves the desktop window", overhanging, clipped)
	}
}
