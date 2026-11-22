//go:build linux

package vision

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	platformlinux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
	"github.com/y3owk1n/neru/internal/adapter/vision/segment"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// Linux answers this port with screen capture (wlr-screencopy on wlroots,
// XGetImage on X11, a PipeWire stream off the portal's ScreenCast session on
// KDE) plus yomi (yomi_read.go) for text and the pure-Go contour detector for
// rectangles, run concurrently over one frame (detect_elements.go).
//
// Privacy: recognized text is screen content. It reaches an element's title
// and search text and nowhere else. Nothing here logs it or counts it into a
// message. Debug lines carry durations and counts.

// captureText captures region for DetectElements and returns the frame, the
// region it shows, which on Linux is region itself, and the scale its text is
// drawn at.
func (a *Adapter) captureText(
	ctx context.Context,
	region image.Rectangle,
) (*image.RGBA, image.Rectangle, int, error) {
	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, image.Rectangle{}, 0, err
	}

	return img, region, yomiScale(img, region), nil
}

// AccurateReadDiffers is false. yomi reads every script in one pass, so an
// accurate read would read the same lines again.
func (a *Adapter) AccurateReadDiffers() bool {
	return false
}

// CaptureExcludesOwnWindows is true once the overlay is built. No Linux
// capture can leave a window out, so the overlay takes itself off the screen
// around each one (platform/linux capture_exclusion.go).
func (a *Adapter) CaptureExcludesOwnWindows() bool {
	return platformlinux.CaptureExcludesOwnWindows()
}

// CaptureScreen returns the pixels currently on the active screen.
func (a *Adapter) CaptureScreen(ctx context.Context) (*image.RGBA, error) {
	return a.captureRegion(ctx, image.Rectangle{})
}

// DetectContours captures screenBounds and runs the contour detector over it.
func (a *Adapter) DetectContours(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsContourConfig,
) ([]*element.Element, error) {
	frame, err := a.captureDetectionFrame(ctx, screenBounds, "contour")
	if err != nil {
		return nil, err
	}

	rects, err := contour.Detect(ctx, frame.img, frame.scale, contour.ParamsFromConfig(cfg))
	if err != nil {
		return nil, err
	}

	return targetElements(frame, rects, element.SourceContour), nil
}

// DetectSegments captures screenBounds and runs the segment detector over it.
func (a *Adapter) DetectSegments(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	frame, err := a.captureDetectionFrame(ctx, screenBounds, "segment")
	if err != nil {
		return nil, err
	}

	return detectSegmentsIn(ctx, frame, cfg)
}

// detectSegmentsIn runs the segment detector on frame.
func detectSegmentsIn(
	ctx context.Context,
	frame detectionFrame,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	rects, err := segment.Detect(ctx, frame.img, frame.scale, cfg.Targets)
	if err != nil {
		return nil, err
	}

	return targetElements(frame, rects, element.SourceSegment), nil
}

// captureDetectionFrame captures just screenBounds. Backends read at the
// output's native resolution, so the scale comes from the frame's height
// against the logical region.
func (a *Adapter) captureDetectionFrame(
	ctx context.Context,
	screenBounds image.Rectangle,
	detector string,
) (detectionFrame, error) {
	region, err := detectionRegion(ctx, screenBounds, detector)
	if err != nil {
		return detectionFrame{}, err
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return detectionFrame{}, err
	}

	return detectionFrameOf(img, region), nil
}

// detectionFrameOf wraps img, a capture of region, for a detector.
func detectionFrameOf(img *image.RGBA, region image.Rectangle) detectionFrame {
	scale := 1.0
	if img.Rect.Dy() > 0 {
		scale = float64(img.Rect.Dy()) / float64(region.Dy())
	}

	return detectionFrame{img: img, origin: region.Min, region: region, scale: scale}
}

// Health reports whether the vision strategy can run on this machine, checking
// both halves without exercising either destructively.
//
// The capture half reads the live backend label rather than taking a frame,
// catching no display server, GNOME, or an unrecognized Wayland compositor.
// It deliberately ignores a KDE session whose screen-sharing consent is not
// yet approved, because that is a permission, answered by
// SystemPort.CheckScreenCapturePermission so the user is asked rather than
// told the strategy is unavailable.
//
// The recognition half loads yomi's recognizers, which are compiled in, so it
// fails only on a broken build. Nothing in the daemon calls this today.
func (a *Adapter) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	captureErr := platformlinux.ScreenCaptureSupported(a.backend())
	if captureErr != nil {
		return captureErr
	}

	_, err := yomiEngine()

	return err
}

// backend names the live display server, from platform.DetectLinuxBackend, the
// one place that answer comes from (internal/adapter/platform/AGENTS.md).
func (a *Adapter) backend() string {
	return platform.DetectLinuxBackend().String()
}

// captureRegion captures region, in global top-left-origin, Y-down, unscaled
// pixels, through the live session's backend. An empty rectangle means the
// whole active screen. The region lets a window-constrained caller avoid
// reading back a whole 4K display. Nothing derived from the image is logged.
func (a *Adapter) captureRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	select {
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	started := time.Now()

	img, err := platformlinux.CaptureScreenRegion(ctx, a.backend(), region)
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

// captureProbe captures region with the overlay left on the screen, for the
// captures that watch the middle of a region settle. The overlay draws nothing
// there that changes while they watch, and taking it off for each would blink
// it every settleInterval.
func (a *Adapter) captureProbe(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	return platformlinux.CaptureScreenRegionWithOverlay(ctx, a.backend(), region)
}

// overlayDrawnOn returns where the overlay drew, which the probes leave out.
func (a *Adapter) overlayDrawnOn() []image.Rectangle {
	return platformlinux.OwnWindowsDrawnOn()
}

// ReadText reports CodeNotSupported. yomi already reads text accurately in the
// detection pass, so copy_text keeps the text the hint has.
func (a *Adapter) ReadText(_ context.Context, _ image.Rectangle) (string, error) {
	return "", derrors.New(derrors.CodeNotSupported, "accurate re-reading is not needed on Linux")
}

// ReadLines captures region and has yomi read it. Both levels read the same
// way, since one pass reads every script accurately.
func (a *Adapter) ReadLines(
	ctx context.Context,
	region image.Rectangle,
	level ports.TextLevel,
) ([][]ports.TextPiece, error) {
	if region.Empty() {
		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"reading lines needs a region with an area",
		)
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	lines, err := a.readFrameLines(ctx, region, img, level)
	if err == nil && level == ports.TextFast {
		a.lastRead.remember(region, img, lines)
	}

	return lines, err
}

// readFrameLines reads img, a capture of region. level only goes to the log,
// since both levels read alike.
func (a *Adapter) readFrameLines(
	ctx context.Context,
	region image.Rectangle,
	img *image.RGBA,
	level ports.TextLevel,
) ([][]ports.TextPiece, error) {
	started := time.Now()

	lines, err := readYomiLines(ctx, img, region, yomiScale(img, region))
	if err != nil {
		return nil, derrors.Wrapf(
			err,
			derrors.GetCode(err),
			"reading lines failed after %s (frame %dx%d)",
			time.Since(started),
			img.Rect.Dx(),
			img.Rect.Dy(),
		)
	}

	a.logger.Debug("Read lines",
		zap.Bool("accurate", level == ports.TextAccurate),
		zap.Int("line_count", len(lines)),
		zap.Duration("recognition", time.Since(started)))

	return lines, nil
}

// accurateReadPreparation is nil. yomi compiles nothing on first use, so
// there is nothing to prepare.
func (a *Adapter) accurateReadPreparation() func() error { return nil }
