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

// Linux holds exactly one half of ports.VisionPort: it can capture the screen,
// and it cannot recognize anything in what it captured. This file pins that
// split, because both halves of it are easy to break in opposite directions.

// TestVisionAdapter_RecognitionStaysNotSupportedOnLinux is the half that
// protects the user.
//
// The hint pipeline picks between the accessibility strategy and the vision
// strategy by calling Health and checking IsNotSupported. Screen capture
// landing makes it tempting to report Health healthy — capture works, after
// all — and the result would be the pipeline selecting vision on Linux, then
// getting nothing back from DetectElements, and the user seeing no hints and no
// error. Health stays not-supported until there is an engine that can read a
// captured frame.
func TestVisionAdapter_RecognitionStaysNotSupportedOnLinux(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	for i := range 3 {
		err := adapter.Health(ctx)
		if !derrors.IsNotSupported(err) {
			t.Fatalf("Health call %d returned %v, want CodeNotSupported every time", i+1, err)
		}
	}

	elements, err := adapter.DetectElements(
		ctx,
		image.Rect(0, 0, 100, 100),
		config.DefaultConfig().Hints.Vision,
		false,
	)
	if !derrors.IsNotSupported(err) {
		t.Errorf("DetectElements returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}

	if elements != nil {
		t.Errorf("DetectElements returned %d elements alongside its error, want nil", len(elements))
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
