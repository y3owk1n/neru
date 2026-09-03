//go:build linux

package vision_test

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Linux implements both halves of ports.VisionPort: screen capture through
// wlr-screencopy or XGetImage, and text recognition through tesseract. Neither
// half can be exercised for real on a runner with no display server and,
// depending on the image, no language data — so this file pins the *shape* of
// every answer, which is where a half-implemented port does its damage.
//
// The rule these tests share: never neither, never both. A method returns a
// result or an error, and an error that means "this machine cannot do this"
// says which piece is missing rather than "vision failed".

// TestVisionAdapter_DetectElementsAnswersOnLinux is the half that protects the
// user. Reporting no elements and no error is how a strategy silently produces
// no hints: the hint pipeline logs nothing a user sees and shows an empty
// overlay. Detection either finds text, finds none because there is none, or
// says what stopped it.
func TestVisionAdapter_DetectElementsAnswersOnLinux(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	elements, err := adapter.DetectElements(
		context.Background(),
		image.Rect(0, 0, 100, 100),
		config.DefaultConfig().Hints.Vision,
		false,
	)

	if err != nil && elements != nil {
		t.Errorf("DetectElements returned %d elements alongside its error %v", len(elements), err)
	}

	if err != nil && strings.Contains(err.Error(), "not implemented on Linux") {
		t.Errorf("DetectElements still refuses as unimplemented: %v", err)
	}
}

// TestVisionAdapter_DetectElementsHonorsDetectTextOff is the one documented
// "neither" answer, and it is worth pinning precisely because it looks like the
// failure the rule above forbids.
//
// Text is the whole of what this backend detects — rectangle detection has no
// OCR equivalent and is declared macOS-only — so hints.vision.detect_text = false
// leaves nothing to run. That is the user's own instruction rather than a
// failure, so it is an empty result and not an error, and it must come back
// without capturing the screen first.
func TestVisionAdapter_DetectElementsHonorsDetectTextOff(t *testing.T) {
	cfg := config.DefaultConfig().Hints.Vision
	cfg.DetectText = false

	elements, err := vision.NewAdapter(nil).DetectElements(
		context.Background(),
		image.Rect(0, 0, 100, 100),
		cfg,
		false,
	)
	if err != nil {
		t.Fatalf("DetectElements failed with text detection off: %v", err)
	}

	if len(elements) != 0 {
		t.Errorf("got %d elements with text detection off, want none", len(elements))
	}
}

// TestVisionAdapter_DetectElementsRefusesAnEmptyRegion pins the one request
// that has no correct answer. The region is what places every result on the
// screen, so an empty one cannot be read as "capture everything" — hints would
// land wherever the frame happened to start.
func TestVisionAdapter_DetectElementsRefusesAnEmptyRegion(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	elements, err := adapter.DetectElements(
		context.Background(),
		image.Rectangle{},
		config.DefaultConfig().Hints.Vision,
		false,
	)
	if err == nil {
		t.Fatalf("DetectElements accepted an empty region and returned %d elements", len(elements))
	}

	if elements != nil {
		t.Error("DetectElements returned elements alongside its error")
	}
}

// TestVisionAdapter_DetectElementsIsCancelable keeps a caller that has given up
// from paying for a capture and a recognition nobody will read. Detection is
// the most expensive call in this adapter by a wide margin.
func TestVisionAdapter_DetectElementsIsCancelable(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	elements, err := adapter.DetectElements(
		ctx,
		image.Rect(0, 0, 100, 100),
		config.DefaultConfig().Hints.Vision,
		false,
	)
	if err == nil {
		t.Fatal("DetectElements ignored a canceled context")
	}

	if elements != nil {
		t.Error("DetectElements returned elements for a canceled context")
	}
}

// TestVisionAdapter_HealthNamesWhatIsMissingOnLinux is the acceptance criterion
// for the pieces no linking decision can settle. The strategy needs two things
// this machine may not have — a display server that can be captured, and
// tesseract language data, which is a package separate from the library Neru
// links — so a build that starts perfectly well can still be unable to run it.
// When that is the case Health must report CodeNotSupported *naming which one*,
// because "the vision strategy is unavailable" is not something a user can act
// on.
func TestVisionAdapter_HealthNamesWhatIsMissingOnLinux(t *testing.T) {
	// The pieces Health can find missing, each named by the word a user would
	// search for. A runner with no session hits the first; a machine with the
	// library and no language pack hits "traineddata"; a CGO_ENABLED=0 build
	// hits "CGO".
	//
	// Which one this run hits is not chosen here: the display backend is
	// detected once per process (platform.DetectLinuxBackend caches it), so a
	// test cannot put this adapter on a different display server than the one
	// the suite is running under. What is pinned is that whichever half is
	// missing, the sentence names it.
	nameable := []string{"display", "screencopy", "traineddata", "CGO"}

	adapter := vision.NewAdapter(nil)

	err := adapter.Health(context.Background())
	if err == nil {
		// A session that can be captured, with the language data installed —
		// the state a machine with the documented dependencies is in.
		return
	}

	if !derrors.IsNotSupported(err) {
		t.Fatalf("Health failed with code %q, want CodeNotSupported so callers degrade: %v",
			derrors.GetCode(err), err)
	}

	for _, piece := range nameable {
		if strings.Contains(err.Error(), piece) {
			return
		}
	}

	t.Errorf("Health refuses without naming any of %v: %v", nameable, err)
}

// TestVisionAdapter_HealthIsRepeatable guards the engine cache behind
// recognition: Health warms tesseract, and a warm-up that answered differently
// the second time would make the strategy's availability depend on how many
// times it had been asked about.
func TestVisionAdapter_HealthIsRepeatable(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	first := derrors.GetCode(adapter.Health(ctx))

	for i := range 2 {
		if got := derrors.GetCode(adapter.Health(ctx)); got != first {
			t.Fatalf("Health call %d reported %q, want %q", i+2, got, first)
		}
	}
}

// TestVisionAdapter_CaptureScreenAnswersOnLinux is the half that protects the
// capability claim. A capture that quietly returned (nil, nil) would read as
// success to every caller and be indistinguishable from a working backend, so
// the contract is: pixels or an error, never neither and never both.
//
// The CI runner has no display server, so this asserts the shape of the answer
// rather than the pixels. A session with a compositor takes the same path and
// lands in the first branch.
func TestVisionAdapter_CaptureScreenAnswersOnLinux(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	img, err := adapter.CaptureScreen(context.Background())

	switch {
	case err == nil && img == nil:
		t.Fatal(
			"CaptureScreen returned no image and no error; a caller cannot tell that apart from success",
		)
	case err == nil:
		if img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
			t.Errorf("CaptureScreen succeeded with an empty %v image", img.Rect)
		}
	case img != nil:
		t.Error("CaptureScreen returned an image alongside its error")
	}

	if err != nil && strings.Contains(err.Error(), "macOS") {
		t.Errorf("CaptureScreen still refuses as a macOS-only capability: %v", err)
	}
}

// TestVisionAdapter_CaptureScreenIsCancelable pins that a capture respects a
// canceled context before it reaches the display server. Capture is the one
// call in this adapter that can take tens of milliseconds, so a caller that has
// given up must not pay for a frame nobody will read.
func TestVisionAdapter_CaptureScreenIsCancelable(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	img, err := adapter.CaptureScreen(ctx)
	if err == nil {
		t.Fatal("CaptureScreen ignored a canceled context")
	}

	if img != nil {
		t.Error("CaptureScreen returned an image for a canceled context")
	}
}

// TestVisionAdapter_NilLoggerIsAcceptedOnLinux covers the constructor contract
// shared with every other adapter in the tree: a nil logger falls back to a
// no-op rather than panicking on first use. Capture is the method that logs, so
// it is the one worth exercising.
func TestVisionAdapter_NilLoggerIsAcceptedOnLinux(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	if adapter == nil {
		t.Fatal("NewAdapter(nil) returned nil")
	}

	_, _ = adapter.CaptureScreen(context.Background())
}
