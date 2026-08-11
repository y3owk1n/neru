//go:build integration && linux

package linux

import (
	"image"
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// These run against a live display server — an Xvfb :display or a headless
// wlroots compositor — and are the only place the native capture bridges are
// actually executed. Everything else about capture can be asserted headless;
// whether XGetImage and wlr-screencopy hand back real pixels cannot.
//
// Run with: go test -tags=integration ./internal/adapter/platform/linux/

// liveCaptureBackend returns the backend label for the session these tests can
// capture from, skipping when there is none.
//
// A Wayland session is assumed to be wlroots-family. This file cannot ask
// platform.DetectLinuxBackend, because internal/adapter/platform imports this
// package and an in-package test cannot import it back; nor may it read the
// desktop-identity environment itself, which platform/AGENTS.md confines to
// backend_linux.go. The guess is harmless because it is not load-bearing: a
// compositor that turns out not to implement screencopy — KWin — answers
// CodeNotSupported and requireCapture skips, so this file reports what the
// session can actually do rather than what it was labeled.
func liveCaptureBackend(t *testing.T) string {
	t.Helper()

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return backendWaylandWlroots
	}

	if os.Getenv("DISPLAY") != "" {
		return backendX11
	}

	t.Skip("no display server: set DISPLAY or WAYLAND_DISPLAY")

	return ""
}

// requireCapture captures region, skipping when this display server has no
// capture path at all and failing when it has one that did not work.
func requireCapture(t *testing.T, backend string, region image.Rectangle) *image.RGBA {
	t.Helper()

	img, err := CaptureScreenRegion(backend, region)
	if derrors.IsNotSupported(err) {
		t.Skipf("this display server cannot capture: %v", err)
	}

	if err != nil {
		t.Fatalf("CaptureScreenRegion(%s, %v) failed: %v", backend, region, err)
	}

	return img
}

// TestCaptureScreenRegion_ReturnsPixels is the acceptance check: a real display
// server, real pixels, in the layout image.RGBA promises.
func TestCaptureScreenRegion_ReturnsPixels(t *testing.T) {
	backend := liveCaptureBackend(t)
	img := requireCapture(t, backend, image.Rectangle{})

	if img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		t.Fatalf("captured an empty image: %v", img.Rect)
	}

	want := img.Rect.Dx() * img.Rect.Dy() * 4
	if len(img.Pix) != want {
		t.Fatalf("captured %d bytes for a %v image, want %d", len(img.Pix), img.Rect, want)
	}

	if img.Stride != img.Rect.Dx()*4 {
		t.Errorf("stride is %d for a %d-wide image; the capture is expected to be tightly packed",
			img.Stride, img.Rect.Dx())
	}

	// Alpha is forced opaque: image.RGBA is alpha-premultiplied, so a captured
	// frame carrying anything else would render darker than the screen it came
	// from.
	for offset := 3; offset < len(img.Pix); offset += 4 {
		if img.Pix[offset] != 0xFF {
			t.Fatalf("pixel %d has alpha %d, want 255", offset/4, img.Pix[offset])
		}
	}
}

// TestCaptureScreenRegion_SurvivesRepeatedUse is the case nothing exercised
// until the vision hint strategy landed.
//
// Before OCR there was no caller that captured more than once in a process, so
// a backend that works on the first frame and not the second would have shipped
// green: every other test here captures once. A hint activation captures every
// time, so "works once" is the shape a capture bug takes for a user, and it is
// the shape this asserts against.
//
// The regions differ deliberately. Both the wlroots client and the X11 path
// resolve an output and validate the rectangle per call, and a state bug that
// only shows when the geometry changes would pass a loop over one rectangle.
func TestCaptureScreenRegion_SurvivesRepeatedUse(t *testing.T) {
	backend := liveCaptureBackend(t)
	full := requireCapture(t, backend, image.Rectangle{})

	regions := []image.Rectangle{
		{},
		image.Rect(0, 0, full.Rect.Dx()/2, full.Rect.Dy()/2),
		image.Rect(0, 0, full.Rect.Dx()/4, full.Rect.Dy()/4),
		{},
	}

	for round := range 2 {
		for index, region := range regions {
			img, err := CaptureScreenRegion(backend, region)
			if err != nil {
				t.Fatalf("round %d region %d (%v): capture %d of this process failed: %v",
					round, index, region, round*len(regions)+index+2, err)
			}

			if img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
				t.Fatalf("round %d region %d (%v): captured an empty %v image",
					round, index, region, img.Rect)
			}
		}
	}
}

// TestCaptureScreenRegion_HonorsTheRegion is the other half of the contract: a
// caller constrained to one window must pay for one window, not for the whole
// display.
func TestCaptureScreenRegion_HonorsTheRegion(t *testing.T) {
	backend := liveCaptureBackend(t)
	full := requireCapture(t, backend, image.Rectangle{})

	region := image.Rect(0, 0, full.Rect.Dx()/2, full.Rect.Dy()/2)
	part := requireCapture(t, backend, region)

	if part.Rect.Dx() >= full.Rect.Dx() || part.Rect.Dy() >= full.Rect.Dy() {
		t.Fatalf("region capture returned %v for %v, which is not smaller than the whole screen %v",
			part.Rect, region, full.Rect)
	}

	// The frame covers exactly the region, so it is never smaller than the
	// logical rectangle asked for. It can be larger: a scaled Wayland output
	// answers in physical pixels.
	if part.Rect.Dx() < region.Dx() || part.Rect.Dy() < region.Dy() {
		t.Errorf("region capture returned %v, smaller than the %v logical region requested",
			part.Rect, region)
	}
}

// TestCaptureScreenRegion_RejectsAPartiallyOffScreenRegion pins the
// exact-region contract at its edge. Clipping would return a frame whose
// top-left is not the caller's, with nothing in the result to say so.
func TestCaptureScreenRegion_RejectsAPartiallyOffScreenRegion(t *testing.T) {
	backend := liveCaptureBackend(t)
	full := requireCapture(t, backend, image.Rectangle{})

	overhanging := image.Rect(full.Rect.Dx()-10, 0, full.Rect.Dx()+200, 100)

	img, err := CaptureScreenRegion(backend, overhanging)
	if err == nil {
		t.Fatalf("capturing %v, which leaves the screen, succeeded and returned %v",
			overhanging, img.Rect)
	}

	if img != nil {
		t.Error("a partially off-screen capture returned an image alongside its error")
	}
}

// TestCaptureScreenRegion_RejectsAnOffScreenRegion pins that a region with no
// pixels behind it fails rather than quietly returning the whole screen.
func TestCaptureScreenRegion_RejectsAnOffScreenRegion(t *testing.T) {
	backend := liveCaptureBackend(t)

	img, err := CaptureScreenRegion(backend, image.Rect(1_000_000, 1_000_000, 1_000_100, 1_000_100))
	if err == nil {
		t.Fatalf("capturing a region off every screen succeeded, returning %v", img.Rect)
	}

	if img != nil {
		t.Error("an off-screen capture returned an image alongside its error")
	}
}
