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

const (
	magenta = 0xFFFF00FF
	cyan    = 0xFF00FFFF
)

// paintRect puts one filled rectangle on the overlay and gives the compositor
// time to show it. It draws through the surface every real draw goes through,
// which records what was drawn, so the hider knows to take it off.
func paintRect(t *testing.T, mgr *Manager, rect image.Rectangle) {
	t.Helper()

	var srf overlaySurface

	switch {
	case mgr.x11 != nil:
		srf = mgr.x11.srf
	case mgr.wlroots != nil:
		srf = mgr.wlroots.srf
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
	return colorShare(img, magenta)
}

// colorShare is the share of pixels in img within a few levels of the opaque
// ARGB color on every channel.
func colorShare(img *image.RGBA, argb uint32) float64 {
	near := func(got uint8, want uint32) bool {
		return max(uint32(got), want)-min(uint32(got), want) < 0x10
	}

	var hits, total int

	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			total++

			pixel := img.RGBAAt(x, y)
			if near(pixel.R, argb>>16&0xFF) && near(pixel.G, argb>>8&0xFF) &&
				near(pixel.B, argb&0xFF) {
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

// drawTestBadge puts a cyan indicator badge with its top-left corner at
// corner, big enough to cover a 100 pixel square, through the draw the mode
// indicator goes through.
func drawTestBadge(mgr *Manager, corner image.Point) {
	colors := overlayColors{background: cyan, border: cyan, text: cyan}
	style := overlayBadgeStyle{fontSize: 40, paddingX: 80, paddingY: 50, borderWidth: 1}

	mgr.renderMu.Lock()
	defer mgr.renderMu.Unlock()

	switch {
	case mgr.x11 != nil:
		mgr.x11.DrawBadge(modeIndicatorBadgeSlot, corner.X, corner.Y, "MM", colors, style)
	case mgr.wlroots != nil:
		mgr.wlroots.DrawBadge(modeIndicatorBadgeSlot, corner.X, corner.Y, "MM", colors, style)
	}

	mgr.modeIndicatorBadgeRect = badgeBounds(
		corner.X,
		corner.Y,
		"MM",
		scaleBadgeFont(style, mgr.overlayScale()),
	)
	mgr.modeIndicatorBadgeVisible = true
}

// TestManagerIndicatorBadge_LeavesWhatTheOverlayDrewAlone pins the bisect
// report. A badge used to be painted onto the overlay and erased by clearing
// its rectangle, so a badge following the cursor across a grid left a trail
// of holes in it. It is a surface of its own now, so moving it leaves what is
// under it as it was, and a capture still leaves it out.
//
// The regions sit right of the one the test above paints, because KWin keeps
// a destroyed overlay's last frame until the process exits.
func TestManagerIndicatorBadge_LeavesWhatTheOverlayDrewAlone(t *testing.T) {
	mgr := NewOverlayManager(zap.NewNop())
	if mgr == nil {
		t.Skip("skipping: no display")
	}

	t.Cleanup(mgr.Destroy)

	backend := platform.DetectLinuxBackend().String()

	painted := image.Rect(500, 0, 900, 400)
	underBadge := image.Rect(600, 100, 700, 200)
	badgeOnly := image.Rect(1000, 100, 1100, 200)

	background := rawCapture(t, mgr, backend, badgeOnly)

	paintRect(t, mgr, painted)

	// Without a compositing manager the X server keeps no window's pixels,
	// so the overlay under a badge that moves away shows its background
	// until it is drawn again. The overlay is not see-through there anyway.
	if mgr.x11 == nil || mgr.x11.composited() {
		drawTestBadge(mgr, underBadge.Min.Sub(image.Pt(40, 40)))
		time.Sleep(600 * time.Millisecond)

		if share := colorShare(rawCapture(t, mgr, backend, underBadge), cyan); share < 0.9 {
			t.Fatalf("only %.2f of the raw copy shows the badge over the overlay, "+
				"so the check below means nothing", share)
		}

		drawTestBadge(mgr, badgeOnly.Min.Sub(image.Pt(40, 40)))
		time.Sleep(600 * time.Millisecond)

		if share := magentaShare(rawCapture(t, mgr, backend, underBadge)); share < 0.9 {
			t.Fatalf("only %.2f of the overlay is left where the badge moved off it", share)
		}
	} else {
		drawTestBadge(mgr, badgeOnly.Min.Sub(image.Pt(40, 40)))
		time.Sleep(600 * time.Millisecond)
	}

	if share := colorShare(rawCapture(t, mgr, backend, badgeOnly), cyan); share < 0.9 {
		t.Fatalf(
			"only %.2f of the raw copy shows the badge, so the check below means nothing",
			share,
		)
	}

	img, err := platformlinux.CaptureScreenRegion(context.Background(), backend, badgeOnly)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%v) through the hider: %v", badgeOnly, err)
	}

	if share := sameShare(img, background); share < 0.9 {
		t.Fatalf("only %.2f of the capture matches what is behind the badge", share)
	}

	time.Sleep(600 * time.Millisecond)

	if share := colorShare(rawCapture(t, mgr, backend, badgeOnly), cyan); share < 0.9 {
		t.Fatalf("only %.2f of the raw copy shows the badge after the capture", share)
	}
}
