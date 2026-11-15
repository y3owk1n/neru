//go:build integration && linux && cgo

package linux

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

var xrandrCurrentSize = regexp.MustCompile(`current \d+ x \d+`)

// setScreenSize asks the X server for a root window of size through xrandr.
// Xvfb offers one size only; Xephyr -resizeable offers several, 800x600 among
// them.
func setScreenSize(ctx context.Context, size image.Point) error {
	sizeArg := fmt.Sprintf("%dx%d", size.X, size.Y)

	out, err := exec.CommandContext(ctx, "xrandr", "-s", sizeArg).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xrandr -s %s: %w: %s", sizeArg, err, out)
	}

	return nil
}

// currentScreenSize reads the root window's size from xrandr. It skips the
// test when xrandr cannot report one.
func currentScreenSize(t *testing.T) image.Point {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "xrandr", "--query").Output()
	if err != nil {
		t.Skipf("skipping: xrandr: %v", err)
	}

	match := xrandrCurrentSize.Find(out)
	if match == nil {
		t.Skipf("skipping: no current screen size in xrandr output: %s", out)
	}

	var size image.Point

	_, err = fmt.Sscanf(string(match), "current %d x %d", &size.X, &size.Y)
	if err != nil {
		t.Fatalf("parsing the screen size: %v", err)
	}

	return size
}

// TestManagerResizeToActiveScreen_CoversARootThatGrewAfterStartup pins the
// overlay following the root window when the desktop grows after the daemon
// started, such as a monitor attached later. The overlay used to keep the size
// it was created with, so anything drawn past it, hints on the new monitor
// included, never showed.
//
// It resizes the X server's screen, so it runs only in the desktop tier, and
// only on a single-monitor display: xrandr -s cannot restore a size spanning
// several monitors. Run it in a nested Xephyr -resizeable that no window
// manager resizes, such as one inside Xvfb.
func TestManagerResizeToActiveScreen_CoversARootThatGrewAfterStartup(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if platform.DetectLinuxBackend() != platform.BackendX11 {
		t.Skip("skipping: needs an X11 session")
	}

	monitors, err := exec.CommandContext(t.Context(), "xrandr", "--listmonitors").Output()
	if err != nil {
		t.Skipf("skipping: xrandr: %v", err)
	}

	if !bytes.HasPrefix(monitors, []byte("Monitors: 1\n")) {
		t.Skip("skipping: resizes the screen, which is undone reliably on one monitor only")
	}

	full := currentScreenSize(t)

	small := image.Pt(800, 600)
	if full.X <= small.X || full.Y <= small.Y {
		t.Skipf("skipping: the screen is %v, not larger than %v", full, small)
	}

	err = setScreenSize(t.Context(), small)
	if err != nil {
		t.Skipf("skipping: the X server cannot shrink the screen: %v", err)
	}

	// t.Context is canceled before cleanups run.
	t.Cleanup(func() {
		restoreErr := setScreenSize(context.Background(), full)
		if restoreErr != nil {
			t.Errorf("restoring the screen size: %v", restoreErr)
		}
	})

	// xrandr reports success even when a window manager keeps the nested
	// server at its old size. The overlay would then start at full size, and
	// the test would pass without the fix.
	if shrunk := currentScreenSize(t); shrunk != small {
		t.Skipf("skipping: the screen stayed %v after asking for %v", shrunk, small)
	}

	mgr := NewOverlayManager(zap.NewNop())
	if mgr == nil || mgr.x11 == nil {
		t.Skip("skipping: no X11 overlay on this display")
	}

	t.Cleanup(mgr.Destroy)

	err = setScreenSize(t.Context(), full)
	if err != nil {
		t.Fatalf("growing the screen back: %v", err)
	}

	corner := image.Rect(full.X-200, full.Y-200, full.X, full.Y)
	paintRect(t, mgr, corner)

	backend := platform.DetectLinuxBackend().String()
	region := corner.Inset(50)

	if share := magentaShare(rawCapture(t, mgr, backend, region)); share < 0.9 {
		t.Fatalf("only %.2f of a rectangle painted past the startup size shows", share)
	}
}
