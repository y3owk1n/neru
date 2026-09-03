//go:build windows

package vision_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Windows implements the capture half of ports.VisionPort (BitBlt off the
// desktop DC) and not the recognition half, so this file pins the shape of
// each answer: capture and contour return a result or a live reason, never
// neither and never both, and never "not supported", because every Windows
// desktop can read its own pixels; recognition and Health refuse loudly, so
// the hint pipeline learns the vision strategy is unavailable here rather than
// showing an empty overlay.
//
// The capture tests here run on a headless CI runner too: they accept a live
// failure (no interactive desktop) and reject only the two answers a caller
// could misread.

// TestVisionAdapter_RecognitionReportsNotSupportedOnWindows is the half that
// protects the user: a nil from DetectElements or Health would select the
// vision strategy and then silently produce no hints.
func TestVisionAdapter_RecognitionReportsNotSupportedOnWindows(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "Health",
			call: func() error { return adapter.Health(ctx) },
		},
		{
			name: "DetectElements",
			call: func() error {
				elements, err := adapter.DetectElements(
					ctx,
					image.Rect(0, 0, 100, 100),
					config.DefaultConfig().Hints.Vision,
					false,
				)
				if elements != nil {
					t.Errorf(
						"DetectElements returned %d elements alongside its error",
						len(elements),
					)
				}

				return err
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if err == nil {
				t.Fatalf("%s returned nil; the hint pipeline would select the vision "+
					"strategy and then silently produce no hints", testCase.name)
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf("%s returned %v (code %q), want CodeNotSupported",
					testCase.name, err, derrors.GetCode(err))
			}
		})
	}
}

// TestVisionAdapter_CaptureAnswersOnWindows pins that capture and contour
// are implemented: they hand back a frame or a live failure, and never the
// CodeNotSupported the `other` slot answers with, which would tell the hint
// pipeline the contour strategy is unavailable on a platform that has it.
func TestVisionAdapter_CaptureAnswersOnWindows(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	img, err := adapter.CaptureScreen(ctx)
	if (img == nil) == (err == nil) {
		t.Fatalf(
			"CaptureScreen returned image=%v err=%v; want exactly one of the two",
			img != nil,
			err,
		)
	}

	if derrors.IsNotSupported(err) {
		t.Errorf(
			"CaptureScreen reports CodeNotSupported on Windows, which has a capture backend: %v",
			err,
		)
	}

	elements, err := adapter.DetectContours(ctx, image.Rect(0, 0, 100, 100))
	if err != nil && elements != nil {
		t.Errorf("DetectContours returned %d elements alongside its error %v", len(elements), err)
	}

	if derrors.IsNotSupported(err) {
		t.Errorf(
			"DetectContours reports CodeNotSupported on Windows, which has a capture backend: %v",
			err,
		)
	}
}

// TestVisionAdapter_DetectContoursRefusesAnEmptyRegion pins the one input that
// must not be widened: an empty rectangle would otherwise be read as "the whole
// screen" by the capture underneath, and a caller asking for a window must
// never receive the display.
func TestVisionAdapter_DetectContoursRefusesAnEmptyRegion(t *testing.T) {
	adapter := vision.NewAdapter(nil)

	elements, err := adapter.DetectContours(context.Background(), image.Rectangle{})
	if err == nil {
		t.Fatalf("DetectContours accepted an empty region and returned %d elements", len(elements))
	}

	if elements != nil {
		t.Error("DetectContours returned elements alongside its error")
	}
}
