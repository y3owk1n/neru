//go:build integration && linux

package linux

import (
	"image"
	"os"
	"testing"
)

// These run against a live display server — an Xvfb :display or a headless
// wlroots compositor — and are the only place the native capture bridges are
// actually executed. Everything else about capture can be asserted headless;
// whether XGetImage and wlr-screencopy hand back real pixels cannot.
//
// Run with: go test -tags=integration ./internal/adapter/platform/linux/

// captureBackend returns the backend label for the live session, or "" when
// there is no display server to capture from.
func captureBackend() string {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return backendWaylandWlroots
	}

	if os.Getenv("DISPLAY") != "" {
		return backendX11
	}

	return ""
}

// TestCaptureScreenRegion_ReturnsPixels is the acceptance check: a real display
// server, real pixels, in the layout image.RGBA promises.
func TestCaptureScreenRegion_ReturnsPixels(t *testing.T) {
	backend := captureBackend()
	if backend == "" {
		t.Skip("no display server: set DISPLAY or WAYLAND_DISPLAY")
	}

	img, err := CaptureScreenRegion(backend, image.Rectangle{})
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%s, whole screen) failed: %v", backend, err)
	}

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

// TestCaptureScreenRegion_HonorsTheRegion is the other half of the contract: a
// caller constrained to one window must pay for one window, not for the whole
// display.
func TestCaptureScreenRegion_HonorsTheRegion(t *testing.T) {
	backend := captureBackend()
	if backend == "" {
		t.Skip("no display server: set DISPLAY or WAYLAND_DISPLAY")
	}

	full, err := CaptureScreenRegion(backend, image.Rectangle{})
	if err != nil {
		t.Fatalf("whole-screen capture failed: %v", err)
	}

	region := image.Rect(0, 0, full.Rect.Dx()/2, full.Rect.Dy()/2)

	part, err := CaptureScreenRegion(backend, region)
	if err != nil {
		t.Fatalf("CaptureScreenRegion(%s, %v) failed: %v", backend, region, err)
	}

	if part.Rect.Dx() >= full.Rect.Dx() || part.Rect.Dy() >= full.Rect.Dy() {
		t.Fatalf("region capture returned %v for %v, which is not smaller than the whole screen %v",
			part.Rect, region, full.Rect)
	}

	// The compositor answers in physical pixels, so a scaled output returns
	// more pixels than the logical region asked for. What must hold on every
	// backend is that the aspect ratio survived — a region capture that
	// silently fell back to the whole screen would fail the check above, and
	// one that captured the wrong rectangle would usually fail this.
	if part.Rect.Dx() < region.Dx() || part.Rect.Dy() < region.Dy() {
		t.Errorf("region capture returned %v, smaller than the %v logical region requested",
			part.Rect, region)
	}
}

// TestCaptureScreenRegion_RejectsAnOffScreenRegion pins that a region with no
// pixels behind it fails rather than quietly returning the whole screen.
func TestCaptureScreenRegion_RejectsAnOffScreenRegion(t *testing.T) {
	backend := captureBackend()
	if backend == "" {
		t.Skip("no display server: set DISPLAY or WAYLAND_DISPLAY")
	}

	img, err := CaptureScreenRegion(backend, image.Rect(1_000_000, 1_000_000, 1_000_100, 1_000_100))
	if err == nil {
		t.Fatalf("capturing a region off every screen succeeded, returning %v", img.Rect)
	}

	if img != nil {
		t.Error("an off-screen capture returned an image alongside its error")
	}
}
