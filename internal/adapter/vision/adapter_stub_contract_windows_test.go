//go:build windows

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

// Windows implements both halves of ports.VisionPort — capture through BitBlt
// off the desktop DC and recognition through Windows.Media.Ocr — so this file
// pins the shape of each answer rather than a refusal: every method returns a
// result or a live reason, never neither and never both. The one
// CodeNotSupported recognition may answer with is a missing OCR language
// pack, and that answer has to say so, because it is the failure a user can
// fix.
//
// The tests here run on a headless CI runner too: capture accepts a live
// failure (no interactive desktop), recognition accepts a missing language
// pack, and both reject only the answers a caller could misread.

// TestVisionAdapter_RecognitionAnswersOnWindows is the half that protects the
// user: a nil from DetectElements or Health would select the vision strategy
// and then silently produce no hints, and an unexplained CodeNotSupported
// would send them to the wrong remedy.
func TestVisionAdapter_RecognitionAnswersOnWindows(t *testing.T) {
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
				if err != nil && elements != nil {
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
				return
			}

			if derrors.IsNotSupported(err) && !strings.Contains(err.Error(), "language") {
				t.Errorf("%s reports CodeNotSupported without naming the OCR language "+
					"pack, which is the only thing Windows can be missing: %v",
					testCase.name, err)
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

// TestVisionAdapter_DetectionRefusesAnEmptyRegion pins the one input that
// must not be widened: an empty rectangle would otherwise be read as "the
// whole screen" by the capture underneath, and a caller asking for a window
// must never receive the display.
func TestVisionAdapter_DetectionRefusesAnEmptyRegion(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	contours, err := adapter.DetectContours(ctx, image.Rectangle{})
	if err == nil {
		t.Fatalf("DetectContours accepted an empty region and returned %d elements", len(contours))
	}

	if contours != nil {
		t.Error("DetectContours returned elements alongside its error")
	}

	elements, err := adapter.DetectElements(
		ctx,
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
