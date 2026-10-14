//go:build integration && darwin

package vision_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// requireScreenCapture skips unless this process may read the screen. CI
// runners have no Screen Recording grant, and a capture refused there is a
// fact about the machine, not the code.
func requireScreenCapture(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if !darwin.CheckScreenCapturePermissions() {
		t.Skip("Screen Recording is not granted to this process; grant it under " +
			"System Settings > Privacy & Security > Screen Recording to run this")
	}
}

// TestVisionAdapter_PixelDetectorsStayInTheirRegion runs both pixel detectors
// over a real region of the main display and checks every target lands inside
// it, whichever capture path the system took.
func TestVisionAdapter_PixelDetectorsStayInTheirRegion(t *testing.T) {
	requireScreenCapture(t)

	adapter := vision.NewAdapter(nil)
	region := image.Rect(100, 100, 900, 600)

	detectors := map[string]func(context.Context) ([]*element.Element, error){
		"segment": func(ctx context.Context) ([]*element.Element, error) {
			return adapter.DetectSegments(ctx, region, config.DefaultConfig().Hints.Segment)
		},
		"contour": func(ctx context.Context) ([]*element.Element, error) {
			return adapter.DetectContours(ctx, region, config.DefaultConfig().Hints.Contour)
		},
	}

	for name, detect := range detectors {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		started := time.Now()

		elements, err := detect(ctx)

		cancel()

		if err != nil {
			t.Fatalf("%s: detection failed: %v", name, err)
		}

		t.Logf("%s: %d targets in %v", name, len(elements), time.Since(started))

		for _, elem := range elements {
			if !elem.Bounds().In(region) {
				t.Errorf("%s: target %v lies outside region %v", name, elem.Bounds(), region)
			}
		}
	}
}
