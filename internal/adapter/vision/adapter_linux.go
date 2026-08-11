//go:build linux

package vision

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	platformlinux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// Linux has the capture half of this port and not the recognition half. The
// pixels come back through wlr-screencopy on wlroots compositors and XGetImage
// on X11; turning them into elements needs an OCR engine, which is a separate
// piece of work and a Known Gaps entry until it lands.
//
// DetectElements and Health therefore keep reporting CodeNotSupported, and that
// is load-bearing rather than tidiness: the hint pipeline decides the vision
// strategy is unavailable by calling Health and checking IsNotSupported. A
// Health that succeeded on the strength of working capture would make the
// pipeline select vision, and the user would get no hints and no explanation.

// DetectElements reports not-supported on Linux: capture works, recognition
// does not exist yet.
func (a *Adapter) DetectElements(
	_ context.Context,
	_ image.Rectangle,
	_ config.HintsVisionConfig,
	_ bool,
) ([]*element.Element, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"vision element detection is not implemented on Linux: the screen can be "+
			"captured, but no OCR engine is linked to read it",
	)
}

// CaptureScreen returns the pixels currently on the active screen.
func (a *Adapter) CaptureScreen(ctx context.Context) (*image.RGBA, error) {
	return a.captureRegion(ctx, image.Rectangle{})
}

// Health reports not-supported on Linux for the reason above: capture alone is
// not the vision strategy, and reporting healthy would select a strategy that
// produces nothing.
func (a *Adapter) Health(_ context.Context) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"the vision strategy is unavailable on Linux: screen capture is implemented "+
			"but no OCR engine is linked to recognize what it captures",
	)
}

// captureRegion captures region — global, top-left origin, Y down, unscaled
// pixels — through whichever backend the live session uses. An empty rectangle
// means the whole active screen, which is what CaptureScreen asks for.
//
// The region exists so a caller constrained to the focused window pays for the
// focused window: reading a 4K display back to examine one window is the
// difference between usable and not.
//
// Nothing derived from the returned image is logged. The debug line below
// carries a duration and the frame's dimensions, which describe the capture
// rather than its contents.
func (a *Adapter) captureRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	select {
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	started := time.Now()

	img, err := platformlinux.CaptureScreenRegion(platform.DetectLinuxBackend().String(), region)
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
