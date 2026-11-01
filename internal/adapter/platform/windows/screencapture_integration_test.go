//go:build integration && windows

package windows

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real GDI capture tests. They need an interactive desktop. A session-0
// service, which is what a headless runner may be, has no desktop to read and
// the overlay refuses to open there, so every test here skips on that answer.
// Run with: go test -tags=integration ./internal/adapter/platform/windows/

// dpiAwarenessContextPerMonitorV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2.
const dpiAwarenessContextPerMonitorV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)-4

// captureTestDPIOnce makes the test process per-monitor-v2 DPI aware, which
// the shipped exe gets from its manifest and a go test binary does not. It is
// what makes the mixed-DPI assertion below mean something. An unaware process
// sees every monitor virtualized to 96 DPI, and the coordinates then agree by
// construction. Failing is fine. It means the context was already set.
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

	// UpdateLayeredWindow hands the frame to the compositor. Give it a vblank
	// or two before reading it back.
	time.Sleep(100 * time.Millisecond)

	return overlay
}

// TestBitBltRegion_ReadsBackAKnownColor is the acceptance check. A rectangle
// painted one color reads back as that color at the expected pixel, in the
// RGBA layout image.RGBA promises. It reads through the raw copy, because the
// painted rectangle is one of Neru's own windows and CaptureScreenRegion
// leaves those out.
func TestBitBltRegion_ReadsBackAKnownColor(t *testing.T) {
	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)
	paintedOverlay(t, painted, 0xFF10C040)

	img, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
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

// TestCaptureScreenRegion_LeavesNerusOwnWindowsOut pins what a capture reads
// while an overlay is drawn. The raw copy sees the painted overlay, and
// CaptureScreenRegion does not, every time, so a pixel detector run while a
// mode is on screen never reads the overlay as part of the interface.
func TestCaptureScreenRegion_LeavesNerusOwnWindowsOut(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)

	background, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	paintedOverlay(t, painted, 0xFFFF00FF)

	var rawElapsed, elapsed time.Duration

	for round := range 20 {
		rawStart := time.Now()

		raw, err := bitBltRegion(painted)
		rawElapsed += time.Since(rawStart)

		if err != nil {
			t.Fatalf("bitBltRegion(%v): %v", painted, err)
		}

		if !isMagenta(raw) {
			t.Fatalf(
				"round %d: the raw copy does not show the painted overlay, so the check below means nothing",
				round,
			)
		}

		start := time.Now()

		img, err := CaptureScreenRegion(context.Background(), painted)
		if err != nil {
			t.Fatalf("CaptureScreenRegion(%v): %v", painted, err)
		}

		elapsed += time.Since(start)

		if share := sameShare(img, background); share < 0.9 {
			t.Fatalf(
				"round %d: only %.2f of the capture matches what is behind the overlay",
				round,
				share,
			)
		}
	}

	t.Logf("a capture took %v on average with the overlay left out, against %v for the raw copy",
		elapsed/20, rawElapsed/20)
}

// TestWithOwnWindowsExcluded_LeavesOutAWindowShownDuringTheCapture pins that
// a Neru window shown while a capture runs, such as an indicator appearing,
// stays out of it. The test hides the window before the capture starts and
// shows it again just before the copy.
func TestWithOwnWindowsExcluded_LeavesOutAWindowShownDuringTheCapture(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)

	background, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	overlay := paintedOverlay(t, painted, 0xFFFF00FF)
	overlay.Hide()
	time.Sleep(100 * time.Millisecond)

	img, err := withOwnWindowsExcluded(context.Background(), func() (*image.RGBA, error) {
		overlay.Show()
		discardCall(procDwmFlush.Call())
		time.Sleep(100 * time.Millisecond)

		return bitBltRegion(painted)
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}

	raw, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	if !isMagenta(raw) {
		t.Fatal("the overlay did not come back on screen, so the check below means nothing")
	}

	if share := sameShare(img, background); share < 0.9 {
		t.Fatalf(
			"only %.2f of the capture matches what is behind the overlay shown during it",
			share,
		)
	}
}

// isMagenta reports whether the center of img is the opaque magenta the
// exclusion tests paint.
func isMagenta(img *image.RGBA) bool {
	pixel := img.RGBAAt(img.Rect.Dx()/2, img.Rect.Dy()/2)

	return pixel.R == 0xFF && pixel.G == 0x00 && pixel.B == 0xFF
}

// sameShare is the share of pixels in got within a few levels of want on
// every channel, so a capture that blacked a window out instead of leaving it
// out fails where the desktop behind it shows.
func sameShare(got, want *image.RGBA) float64 {
	const tolerance = 8

	near := func(a, b uint8) bool {
		return max(a, b)-min(a, b) <= tolerance
	}

	var same, total int

	for y := got.Rect.Min.Y; y < got.Rect.Max.Y; y++ {
		for x := got.Rect.Min.X; x < got.Rect.Max.X; x++ {
			total++

			g, w := got.RGBAAt(x, y), want.RGBAAt(x, y)
			if near(g.R, w.R) && near(g.G, w.G) && near(g.B, w.B) {
				same++
			}
		}
	}

	return float64(same) / float64(total)
}

// TestSetOwnWindowsHiddenFromCapture_KeepsTheOverlayOutOfEveryCapture pins
// hiding from screen shares. While it is on, the raw copy, which stands in for
// a screen share or screenshot, leaves out an overlay opened afterwards, and a
// capture does not undo that. Once it is off, the overlay is back in the raw
// copy.
func TestSetOwnWindowsHiddenFromCapture_KeepsTheOverlayOutOfEveryCapture(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)

	background, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	err = SetOwnWindowsHiddenFromCapture(true)
	if err != nil {
		t.Fatalf("SetOwnWindowsHiddenFromCapture(true): %v", err)
	}

	t.Cleanup(func() {
		err := SetOwnWindowsHiddenFromCapture(false)
		if err != nil {
			t.Errorf("SetOwnWindowsHiddenFromCapture(false): %v", err)
		}
	})

	paintedOverlay(t, painted, 0xFFFF00FF)

	for _, step := range []string{"opened while hidden", "after a capture"} {
		raw, err := bitBltRegion(painted)
		if err != nil {
			t.Fatalf("bitBltRegion(%v): %v", painted, err)
		}

		if share := sameShare(raw, background); share < 0.9 {
			t.Fatalf(
				"%s: only %.2f of the raw copy matches what is behind the overlay",
				step, share,
			)
		}

		_, err = CaptureScreenRegion(context.Background(), painted)
		if err != nil {
			t.Fatalf("CaptureScreenRegion(%v): %v", painted, err)
		}
	}

	err = SetOwnWindowsHiddenFromCapture(false)
	if err != nil {
		t.Fatalf("SetOwnWindowsHiddenFromCapture(false): %v", err)
	}

	discardCall(procDwmFlush.Call())
	time.Sleep(100 * time.Millisecond)

	raw, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	if !isMagenta(raw) {
		t.Fatal("the overlay did not come back into the raw copy once hiding was off")
	}
}

// TestSetOwnWindowsHiddenFromCapture_HidesAnOverlayAlreadyShown pins that
// hiding waits for the compositor before it returns. An overlay already on
// screen is out of the next copy, so a capture queued behind the change
// cannot read it.
func TestSetOwnWindowsHiddenFromCapture_HidesAnOverlayAlreadyShown(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)

	background, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	paintedOverlay(t, painted, 0xFFFF00FF)

	raw, err := bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	if !isMagenta(raw) {
		t.Fatal("the raw copy does not show the painted overlay, so the check below means nothing")
	}

	err = SetOwnWindowsHiddenFromCapture(true)
	if err != nil {
		t.Fatalf("SetOwnWindowsHiddenFromCapture(true): %v", err)
	}

	t.Cleanup(func() {
		err := SetOwnWindowsHiddenFromCapture(false)
		if err != nil {
			t.Errorf("SetOwnWindowsHiddenFromCapture(false): %v", err)
		}
	})

	raw, err = bitBltRegion(painted)
	if err != nil {
		t.Fatalf("bitBltRegion(%v): %v", painted, err)
	}

	if share := sameShare(raw, background); share < 0.9 {
		t.Fatalf(
			"only %.2f of the copy right after hiding matches what is behind the overlay",
			share,
		)
	}
}

// TestCaptureScreenRegion_ConcurrentCapturesLeaveTheOverlayOut pins why the
// captures take turns. Two captures running at once must both leave the
// overlay out, so one restoring the affinity never uncovers it while the other
// is still reading.
func TestCaptureScreenRegion_ConcurrentCapturesLeaveTheOverlayOut(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)
	paintedOverlay(t, painted, 0xFFFF00FF)

	var (
		wait     sync.WaitGroup
		failedMu sync.Mutex
		failed   []string
	)

	report := func(message string) {
		failedMu.Lock()
		defer failedMu.Unlock()

		failed = append(failed, message)
	}

	for capturer := range 2 {
		wait.Go(func() {
			for round := range 10 {
				img, captureErr := CaptureScreenRegion(context.Background(), painted)
				if captureErr != nil {
					report(captureErr.Error())

					return
				}

				if isMagenta(img) {
					report(fmt.Sprintf("capturer %d round %d read the overlay", capturer, round))
				}
			}
		})
	}

	wait.Wait()

	if len(failed) > 0 {
		t.Fatalf("concurrent captures: %v", failed)
	}
}

// TestCaptureScreenRegion_DoesNotWaitOnTheOverlayThread pins that leaving the
// overlay out of a capture never waits on the thread that owns its windows. A
// capture can run under the mode handler's lock, and the overlay thread can be
// busy, so a capture that waited on it would stall the handler.
func TestCaptureScreenRegion_DoesNotWaitOnTheOverlayThread(t *testing.T) {
	if !CaptureExcludesOwnWindows() {
		t.Skip("skipping: leaving windows out of a capture needs Windows 10 version 2004")
	}

	screen, err := activeScreenBounds()
	if err != nil {
		t.Skipf("no active screen: %v", err)
	}

	painted := image.Rect(screen.Min.X+40, screen.Min.Y+40, screen.Min.X+140, screen.Min.Y+140)
	paintedOverlay(t, painted, 0xFFFF00FF)

	parked := make(chan struct{})
	release := make(chan struct{})

	postOnOverlayUI(func() {
		close(parked)
		<-release
	})

	<-parked

	defer close(release)

	done := make(chan error, 1)

	go func() {
		_, captureErr := CaptureScreenRegion(context.Background(), painted)
		done <- captureErr
	}()

	select {
	case captureErr := <-done:
		if captureErr != nil {
			t.Fatalf("CaptureScreenRegion: %v", captureErr)
		}
	case <-time.After(time.Second):
		t.Fatal("the capture waited on the parked overlay thread")
	}
}

// TestCaptureScreenRegion_MatchesEnumDisplayMonitors pins the DPI contract. On
// a mixed-DPI arrangement, capturing each monitor's reported bounds yields a
// frame of exactly those dimensions. Under per-monitor-v2 both answers are
// physical pixels. A process that lost that awareness would read a scaled
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
// contract. A rectangle that leaves every monitor fails rather than coming
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

// TestClipToScreen_KeepsAWindowOnScreen pins the clip the focused window gets.
// The bounds a capture strategy is handed never leave the virtual screen. It
// uses the desktop window, which every session has and which spans the
// primary monitor exactly, widened by the border a maximized window overhangs.
func TestClipToScreen_KeepsAWindowOnScreen(t *testing.T) {
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

	screen, err := virtualScreenBounds()
	if err != nil {
		t.Skipf("no monitors: %v", err)
	}

	overhanging := rectToImage(rect).Inset(-8)

	clipped, found, err := clipToScreen(overhanging)
	if err != nil {
		t.Fatalf("clipToScreen: %v", err)
	}

	if !found {
		t.Fatal("clipToScreen reported no window for the desktop")
	}

	if !clipped.In(screen) {
		t.Errorf("clipped %v to %v, which still leaves the screen %v", overhanging, clipped, screen)
	}
}
