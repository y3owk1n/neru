//go:build !darwin && !linux && !windows

package vision_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// The vision adapter is fully stubbed on the platforms with neither half of
// the port: no capture backend and no recognition engine, so every method
// reports CodeNotSupported. Linux and Windows have capture and are pinned
// separately by adapter_stub_contract_linux_test.go and
// adapter_stub_contract_windows_test.go.
//
// Pinning that matters more than it looks. The hint pipeline chooses between
// the accessibility strategy and the vision strategy at runtime, and it decides
// vision is unavailable by calling Health and checking IsNotSupported. If a
// stub here started returning nil — say a refactor gave Health a default
// "return nil" — the pipeline would select the vision strategy here, then get
// an empty element list from DetectElements and show the user no hints at all,
// with no error to explain why.
//
// Tagged for the `other` slot only, so no CI leg runs it today; it is the
// contract a fourth platform would inherit.
func TestVisionAdapter_AllMethodsReportNotSupportedOffDarwin(t *testing.T) {
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
			name: "CaptureScreen",
			call: func() error {
				_, err := adapter.CaptureScreen(ctx)

				return err
			},
		},
		{
			name: "DetectElements",
			call: func() error {
				_, err := adapter.DetectElements(
					ctx,
					image.Rect(0, 0, 100, 100),
					config.DefaultConfig().Hints.Vision,
					false,
				)

				return err
			},
		},
		{
			name: "DetectContours",
			call: func() error {
				_, err := adapter.DetectContours(ctx, image.Rect(0, 0, 100, 100))

				return err
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if err == nil {
				t.Fatalf(
					"%s returned nil; the hint pipeline would select the vision "+
						"strategy and then silently produce no hints",
					testCase.name,
				)
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf("%s returned %v (code %q), want CodeNotSupported",
					testCase.name, err, derrors.GetCode(err))
			}
		})
	}
}

// TestVisionAdapter_StubsReturnNoPartialResults checks the stubs hand back
// nothing alongside their error. A caller that ignored the error and used the
// result would otherwise act on a half-built value.
func TestVisionAdapter_StubsReturnNoPartialResults(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	elements, err := adapter.DetectElements(
		ctx,
		image.Rect(0, 0, 100, 100),
		config.DefaultConfig().Hints.Vision,
		true,
	)
	if err == nil {
		t.Fatal("DetectElements returned a nil error")
	}

	if elements != nil {
		t.Errorf("DetectElements returned %d elements alongside its error, want nil",
			len(elements))
	}

	contours, err := adapter.DetectContours(ctx, image.Rect(0, 0, 100, 100))
	if err == nil {
		t.Fatal("DetectContours returned a nil error")
	}

	if contours != nil {
		t.Errorf("DetectContours returned %d elements alongside its error, want nil",
			len(contours))
	}

	capture, err := adapter.CaptureScreen(ctx)
	if err == nil {
		t.Fatal("CaptureScreen returned a nil error")
	}

	if capture != nil {
		t.Error("CaptureScreen returned an image alongside its error, want nil")
	}
}

// TestVisionAdapter_StubsAreStableAcrossCalls guards against a stub that
// succeeds once — for example one that lazily initializes some state and then
// reports a different error, or none, on a second call.
func TestVisionAdapter_StubsAreStableAcrossCalls(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	ctx := context.Background()

	for i := range 3 {
		err := adapter.Health(ctx)
		if !derrors.IsNotSupported(err) {
			t.Fatalf("Health call %d returned %v, want CodeNotSupported every time", i+1, err)
		}
	}
}

// TestVisionAdapter_NilLoggerIsAccepted covers the constructor contract shared
// with every other adapter in the tree: a nil logger falls back to a no-op
// rather than panicking on first use.
func TestVisionAdapter_NilLoggerIsAccepted(t *testing.T) {
	adapter := vision.NewAdapter(nil)
	if adapter == nil {
		t.Fatal("NewAdapter(nil) returned nil")
	}

	// Exercising a method proves the fallback logger is actually usable.
	err := adapter.Health(context.Background())
	if err == nil {
		t.Error("Health returned nil")
	}
}
