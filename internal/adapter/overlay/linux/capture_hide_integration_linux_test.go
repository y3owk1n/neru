//go:build integration && linux && cgo

package linux

import (
	"context"
	"image"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	platformlinux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

const magenta = 0xFFFF00FF

// paintRect puts one filled rectangle on the overlay, through the same
// primitives every draw ends in, and gives the compositor time to show it.
func paintRect(t *testing.T, mgr *Manager, rect image.Rectangle) {
	t.Helper()

	var srf overlaySurface

	switch {
	case mgr.x11 != nil:
		srf = mgr.x11
	case mgr.wlroots != nil:
		srf = mgr.wlroots
	default:
		t.Skip("skipping: no overlay backend on this display")
	}

	mgr.ResizeToActiveScreen()
	mgr.Show()

	mgr.renderMu.Lock()
	srf.ensureBuffers()

	if !srf.beginFrame() {
		mgr.renderMu.Unlock()
		t.Fatal("no buffer to draw the rectangle into")
	}

	srf.clearFrame()
	srf.rectPrim(rect, magenta, magenta, 0)
	srf.surfaceFlush()
	mgr.renderMu.Unlock()

	time.Sleep(600 * time.Millisecond)
}

// rawCapture reads the screen with no overlay hider registered, the way a
// screen share or screenshot sees it, then registers mgr's hider again.
func rawCapture(t *testing.T, mgr *Manager, backend string, region image.Rectangle) *image.RGBA {
	t.Helper()

	platformlinux.SetOwnWindowsHider(nil)
	defer platformlinux.SetOwnWindowsHider(mgr.hideForCapture)

	img, err := platformlinux.CaptureScreenRegion(context.Background(), backend, region)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%v): %v", region, err)
	}

	return img
}

func magentaShare(img *image.RGBA) float64 {
	var hits, total int

	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			total++

			pixel := img.RGBAAt(x, y)
			if pixel.R > 0xF0 && pixel.G < 0x10 && pixel.B > 0xF0 {
				hits++
			}
		}
	}

	return float64(hits) / float64(total)
}

// sameShare is the share of pixels in got within a few levels of want on
// every channel, so a capture that blacked the overlay out instead of leaving
// it out fails where the desktop behind it shows.
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

// TestManagerHideForCapture_LeavesTheOverlayOutAndPutsItBack pins what a
// capture through the hider sees. The raw copy shows a painted overlay, the
// capture shows what is behind it, and afterwards the raw copy shows the
// overlay again, unchanged.
//
// Run it once per process. KWin keeps a destroyed overlay's last frame on
// screen until its client exits, so a second run in the same process starts
// with the first run's rectangle still showing.
func TestManagerHideForCapture_LeavesTheOverlayOutAndPutsItBack(t *testing.T) {
	mgr := NewOverlayManager(zap.NewNop())
	if mgr == nil {
		t.Skip("skipping: no display")
	}

	t.Cleanup(mgr.Destroy)

	backend := platform.DetectLinuxBackend().String()

	// The rectangle is painted in logical pixels and captured in physical
	// ones, so the capture sits well inside it at any scale up to 2.
	painted := image.Rect(0, 0, 400, 400)
	region := image.Rect(100, 100, 200, 200)

	background := rawCapture(t, mgr, backend, region)

	paintRect(t, mgr, painted)

	if share := magentaShare(rawCapture(t, mgr, backend, region)); share < 0.9 {
		t.Fatalf(
			"only %.2f of the raw copy shows the painted overlay, so the check below means nothing",
			share,
		)
	}

	img, err := platformlinux.CaptureScreenRegion(context.Background(), backend, region)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%v) through the hider: %v", region, err)
	}

	if share := sameShare(img, background); share < 0.9 {
		t.Fatalf("only %.2f of the capture matches what is behind the overlay", share)
	}

	time.Sleep(600 * time.Millisecond)

	if share := magentaShare(rawCapture(t, mgr, backend, region)); share < 0.9 {
		t.Fatalf("only %.2f of the raw copy shows the overlay after the capture", share)
	}
}
