//go:build windows

package vision

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	platformwindows "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// Windows answers the capture half of this port with GDI (BitBlt off the
// desktop DC, platform/windows/screencapture.go) and has no recognition
// engine yet, so the contour strategy runs here and the vision strategy does
// not. Every method below either returns a result or says which half stopped
// it, never neither and never both; adapter_stub_contract_windows_test.go pins
// that shape.
//
// Privacy: captured pixels are screen content. They reach the contour
// detector and nowhere else; the debug lines carry durations and dimensions.

// DetectElements reports not-supported: text recognition has no engine on
// Windows yet, and the hint pipeline reads this as "the vision strategy is
// unavailable here" rather than as an empty screen.
func (a *Adapter) DetectElements(
	_ context.Context,
	_ image.Rectangle,
	_ config.HintsVisionConfig,
	_ bool,
) ([]*element.Element, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"vision element detection needs a text recognition engine, which Windows "+
			"does not have yet; use the contour or axtree strategy",
	)
}

// CaptureScreen returns the pixels currently on the active screen, the
// monitor under the cursor.
func (a *Adapter) CaptureScreen(ctx context.Context) (*image.RGBA, error) {
	return a.captureRegion(ctx, image.Rectangle{})
}

// DetectContours captures screenBounds and runs the contour detector over it.
//
// screenBounds is where the caller wants hints, normally the focused window,
// in global top-left-origin unscaled pixels. It places the results as well as
// bounding the capture, so an empty rectangle is refused rather than read as
// "the whole screen".
func (a *Adapter) DetectContours(
	ctx context.Context,
	screenBounds image.Rectangle,
) ([]*element.Element, error) {
	select {
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	region := screenBounds.Canon()
	if region.Empty() {
		return nil, derrors.Newf(
			derrors.CodeActionFailed,
			"contour detection needs a region to read; %v is empty",
			screenBounds,
		)
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	// The process is per-monitor-v2 DPI aware, so the frame is the region's own
	// size in physical pixels and the scale is one.
	rects, err := contour.Detect(img, 1)
	if err != nil {
		return nil, err
	}

	return contour.Elements(region.Min, region, rects), nil
}

// Health reports not-supported: the vision strategy is text recognition, and
// Windows has no engine for it yet. Capture being available is what lets the
// contour strategy run, and that strategy does not consult Health.
func (a *Adapter) Health(_ context.Context) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"the vision strategy needs a text recognition engine, which Windows does not have yet",
	)
}

// captureRegion captures region, global top-left origin, Y down, unscaled
// pixels, off the desktop. An empty rectangle means the whole active screen,
// which is what CaptureScreen asks for.
//
// Nothing derived from the returned image is logged. The debug line below
// carries a duration and the frame's dimensions, which describe the capture
// rather than its contents.
func (a *Adapter) captureRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	started := time.Now()

	img, err := platformwindows.CaptureScreenRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	a.logger.Debug("Captured screen region",
		zap.Duration("duration", time.Since(started)),
		zap.Int("width", img.Rect.Dx()),
		zap.Int("height", img.Rect.Dy()),
	)

	return img, nil
}
