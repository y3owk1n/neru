//go:build darwin

package vision

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreGraphics -framework Foundation
#include "../platform/darwin/vision.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"image"
	"math"
	"time"
	"unsafe"

	"go.uber.org/zap"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin" // ensure darwin CGo .m files are compiled
	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
	"github.com/y3owk1n/neru/internal/adapter/vision/frame"
	"github.com/y3owk1n/neru/internal/adapter/vision/segment"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

const bytesPerPixel = 4

// captureText captures region at readLinesScale for DetectElements and
// returns the frame, the region it shows (region clipped to the display that
// holds it) and the scale its text is drawn at.
func (a *Adapter) captureText(
	ctx context.Context,
	region image.Rectangle,
) (*image.RGBA, image.Rectangle, int, error) {
	frame, err := captureReadFrame(ctx, region)
	if err != nil {
		return nil, image.Rectangle{}, 0, err
	}
	defer releaseFrame(frame)

	img, err := cgImageToRGBA(frame.image)
	if err != nil {
		return nil, image.Rectangle{}, 0, err
	}

	shown := rectOf(frame.bounds)

	return img, shown, yomiScale(img, shown), nil
}

// rectOf is r in integer points.
func rectOf(r C.CGRect) image.Rectangle {
	return image.Rect(
		int(math.Round(float64(r.origin.x))),
		int(math.Round(float64(r.origin.y))),
		int(math.Round(float64(r.origin.x+r.size.width))),
		int(math.Round(float64(r.origin.y+r.size.height))),
	)
}

// AccurateReadDiffers is false. yomi reads every script in one pass, so an
// accurate read would read the same lines again.
func (a *Adapter) AccurateReadDiffers() bool { return false }

// CaptureExcludesOwnWindows is true on macOS, where every capture asks
// ScreenCaptureKit to leave Neru's application out (vision_darwin.m).
func (a *Adapter) CaptureExcludesOwnWindows() bool { return true }

// CaptureScreen captures the current screen image for the primary display.
func (a *Adapter) CaptureScreen(_ context.Context) (*image.RGBA, error) {
	cgImage := C.NeruCaptureScreen()
	if uintptr(cgImage) == 0 {
		return nil, derrors.New(derrors.CodeInternal, "failed to capture screen")
	}

	defer C.CGImageRelease(cgImage)

	return cgImageToRGBA(cgImage)
}

// cgImageToRGBA copies a CGImage into an RGBA buffer at its native pixel size.
func cgImageToRGBA(cgImage C.CGImageRef) (*image.RGBA, error) {
	return cgImageToFrame(cgImage, 1)
}

// cgImageToFrame copies a CGImage into an RGBA buffer, box-filtered down by
// factor. The image is drawn into Go memory as BGRA in its own color space,
// which is the layout ScreenCaptureKit hands out, so Core Graphics copies
// rather than converts, and frame.Resample then swizzles and filters it in one
// pass across cores. No native copy of the pixels is made, so there is none
// to wipe.
func cgImageToFrame(cgImage C.CGImageRef, factor int) (*image.RGBA, error) {
	width := int(C.CGImageGetWidth(cgImage))
	height := int(C.CGImageGetHeight(cgImage))

	if width == 0 || height == 0 {
		return nil, derrors.New(derrors.CodeInternal, "captured screen image has zero size")
	}

	stride := width * bytesPerPixel
	pix := make([]byte, stride*height)

	colorSpace := C.CGImageGetColorSpace(cgImage)
	if uintptr(colorSpace) == 0 || C.CGColorSpaceGetModel(colorSpace) != C.kCGColorSpaceModelRGB {
		colorSpace = C.CGColorSpaceCreateDeviceRGB()
		defer C.CGColorSpaceRelease(colorSpace)
	}

	ctx := C.NeruCreateBGRABitmapContext(
		unsafe.Pointer(&pix[0]),
		C.size_t(width),
		C.size_t(height),
		C.size_t(stride),
		colorSpace,
	)

	if uintptr(ctx) == 0 {
		return nil, derrors.New(derrors.CodeInternal, "failed to create bitmap context")
	}
	defer C.CGContextRelease(ctx)

	C.CGContextDrawImage(ctx, C.CGRect{
		origin: C.CGPoint{x: 0, y: 0},
		size:   C.CGSize{width: C.double(width), height: C.double(height)},
	}, cgImage)

	return frame.Resample(pix, stride, width, height, frame.BGRA, factor), nil
}

// Health checks that the screen can be captured. Text recognition is
// compiled in.
func (a *Adapter) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Quick smoke test: attempt a screen capture
	_, err := a.CaptureScreen(ctx)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeInternal, "screen capture health check failed")
	}

	return nil
}

// DetectContours captures the display holding screenBounds and runs the
// contour detector over it.
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

// DetectSegments captures the display holding screenBounds and runs the
// segment detector over it.
func (a *Adapter) DetectSegments(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	frame, err := a.captureDetectionFrame(ctx, screenBounds, "segment")
	if err != nil {
		return nil, err
	}

	rects, err := segment.Detect(ctx, frame.img, frame.scale, cfg.Targets)
	if err != nil {
		return nil, err
	}

	return targetElements(frame, rects, element.SourceSegment), nil
}

// DetectSegmentsAsRead captures afresh, as DetectSegments does. The read keeps
// a frame at twice the size the detector was tuned on, and a macOS capture
// leaves the overlay out, so reusing the frame would save nothing.
func (a *Adapter) DetectSegmentsAsRead(
	ctx context.Context,
	region image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	return a.DetectSegments(ctx, region, cfg)
}

// captureDetectionFrame captures screenBounds for a pixel detector.
//
// It captures just the region, clipped to the display that holds it, at its
// size in points. The detectors were tuned on logical pixels, and contour on a
// 2x frame finds half again as many targets at a fifth of the speed. When that
// path is unavailable it captures the whole display containing the region
// instead. Either way the detectors' rectangles are offset by where the frame
// sits (CGDisplayBounds is already global top-left, so no flip is needed) and
// trimmed to screenBounds.
func (a *Adapter) captureDetectionFrame(
	ctx context.Context,
	screenBounds image.Rectangle,
	detector string,
) (detectionFrame, error) {
	region, err := detectionRegion(ctx, screenBounds, detector)
	if err != nil {
		return detectionFrame{}, err
	}

	cgRect := C.CGRect{
		origin: C.CGPoint{x: C.double(region.Min.X), y: C.double(region.Min.Y)},
		size:   C.CGSize{width: C.double(region.Dx()), height: C.double(region.Dy())},
	}

	started := time.Now()

	var captured C.CGRect

	timeout, err := captureTimeoutMS(ctx)
	if err != nil {
		return detectionFrame{}, err
	}

	if regionImage := C.NeruCaptureRect(cgRect, &captured, timeout); uintptr(regionImage) != 0 {
		defer C.CGImageRelease(regionImage)

		capturedWidth := max(int(captured.size.width), 1)
		factor := int(math.Round(float64(C.CGImageGetWidth(regionImage)) / float64(capturedWidth)))

		img, err := cgImageToFrame(regionImage, factor)
		if err != nil {
			return detectionFrame{}, err
		}

		a.logCapture("region", started, img)

		return detectionFrame{
			img:    img,
			origin: image.Pt(int(captured.origin.x), int(captured.origin.y)),
			region: region,
			scale:  float64(img.Rect.Dx()) / float64(capturedWidth),
		}, nil
	}

	// The region capture failed or ran out of time. The fallback gets only
	// what is left of the caller's deadline, so with one set the two never
	// wait longer than it allows.
	timeout, err = captureTimeoutMS(ctx)
	if err != nil {
		return detectionFrame{}, err
	}

	var displayBounds C.CGRect

	cgImage := C.NeruCaptureDisplayContaining(cgRect, &displayBounds, timeout)
	if uintptr(cgImage) == 0 {
		return detectionFrame{}, derrors.New(derrors.CodeInternal, "failed to capture display")
	}

	defer C.CGImageRelease(cgImage)

	img, err := cgImageToRGBA(cgImage)
	if err != nil {
		return detectionFrame{}, err
	}

	a.logCapture("display", started, img)

	scale := 1.0
	if displayBounds.size.height > 0 {
		scale = float64(img.Rect.Dy()) / float64(displayBounds.size.height)
	}

	return detectionFrame{
		img:    img,
		origin: image.Pt(int(displayBounds.origin.x), int(displayBounds.origin.y)),
		region: region,
		scale:  scale,
	}, nil
}

// defaultCaptureTimeout bounds a capture whose context carries no deadline.
const defaultCaptureTimeout = 5 * time.Second

// captureTimeoutMS is how long the native capture may wait: what is left on
// ctx, or defaultCaptureTimeout when ctx has no deadline. The native side
// cannot read a Go context, so this is the bound it does read. A context that
// is already done is refused before anything is captured.
func captureTimeoutMS(ctx context.Context) (C.int64_t, error) {
	err := ctx.Err()
	if err != nil {
		return 0, derrors.Wrap(err, derrors.CodeContextCanceled, "operation canceled")
	}

	remaining := defaultCaptureTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining = time.Until(deadline)
	}

	return C.int64_t(max(remaining.Milliseconds(), 1)), nil
}

// logCapture records how a detection frame was taken. It carries a duration
// and the frame's dimensions, which describe the capture rather than its
// contents.
func (a *Adapter) logCapture(path string, started time.Time, img *image.RGBA) {
	a.logger.Debug("Captured detection frame",
		zap.String("path", path),
		zap.Duration("duration", time.Since(started)),
		zap.Int("width", img.Rect.Dx()),
		zap.Int("height", img.Rect.Dy()),
	)
}

// ReadText reports CodeNotSupported. yomi already reads text accurately in the
// detection pass, so copy_text keeps the text the hint has.
func (a *Adapter) ReadText(_ context.Context, _ image.Rectangle) (string, error) {
	return "", derrors.New(derrors.CodeNotSupported, "accurate re-reading is not needed on macOS")
}

// readLinesScale is how much larger than its size in points a region is
// captured for ReadLines, which is a Retina display's own resolution.
const readLinesScale = 2

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

	started := time.Now()

	lines, err := a.readLinesHere(ctx, region, level)
	if err != nil {
		return nil, err
	}

	a.logger.Debug("Read lines",
		zap.Bool("accurate", level == ports.TextAccurate),
		zap.Int("line_count", len(lines)),
		zap.Duration("duration", time.Since(started)))

	return lines, nil
}

// readLinesHere captures region and reads it in this process, the two within
// ctx's one deadline. A fast read is remembered, for ReadLinesOnceChanged to
// wait for the region to change from.
func (a *Adapter) readLinesHere(
	ctx context.Context,
	region image.Rectangle,
	level ports.TextLevel,
) ([][]ports.TextPiece, error) {
	frame, err := captureReadFrame(ctx, region)
	if err != nil {
		return nil, err
	}
	defer releaseFrame(frame)

	lines, err := readFrameLines(ctx, frame)
	if err == nil && level == ports.TextFast {
		a.lastRead.remember(region, frame, lines)
	}

	return lines, err
}

// readFrameLines reads a capture's text with yomi.
func readFrameLines(ctx context.Context, frame readFrame) ([][]ports.TextPiece, error) {
	img, err := cgImageToRGBA(frame.image)
	if err != nil {
		return nil, err
	}

	shown := rectOf(frame.bounds)

	return readYomiLines(ctx, img, shown, yomiScale(img, shown))
}

// ReadLinesOnceChanged waits for region to change from what the last fast
// read of it captured and to settle, then reads it fast. One that never
// changes reads the last read's lines again.
func (a *Adapter) ReadLinesOnceChanged(
	ctx context.Context,
	region image.Rectangle,
) ([][]ports.TextPiece, error) {
	if region.Empty() {
		return a.ReadLines(ctx, region, ports.TextFast)
	}

	started := time.Now()

	before, lines, remembered := a.lastRead.recall(region)
	defer releaseFrame(before)

	frame, unchanged, err := captureOnceChanged(ctx, before, remembered, settleFrames[readFrame]{
		capture: func() (readFrame, error) { return captureReadFrame(ctx, region) },
		same:    sameFrame,
		release: releaseFrame,
	})
	if err != nil {
		return nil, err
	}
	defer releaseFrame(frame)

	if !unchanged {
		lines, err = readFrameLines(ctx, frame)
		if err != nil {
			return nil, err
		}

		a.lastRead.remember(region, frame, lines)
	}

	a.logger.Debug("Read lines once changed",
		zap.Bool("unchanged", unchanged),
		zap.Int("line_count", len(lines)),
		zap.Duration("duration", time.Since(started)))

	return lines, nil
}

func cRect(r image.Rectangle) C.CGRect {
	return C.CGRect{
		origin: C.CGPoint{x: C.double(r.Min.X), y: C.double(r.Min.Y)},
		size:   C.CGSize{width: C.double(r.Dx()), height: C.double(r.Dy())},
	}
}

// accurateReadPreparation is nil. yomi compiles nothing on first use, so
// there is nothing to prepare.
func (a *Adapter) accurateReadPreparation() func() error { return nil }
