//go:build darwin

package vision

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Vision -framework CoreGraphics -framework Foundation
#include "../platform/darwin/vision.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"image"
	"math"
	"sync"
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

// DetectElements captures a screenshot of the main display and runs Vision
// Framework detection (text recognition + rectangle detection). Results are
// merged via non-maximum suppression and classified by the heuristic classifier.
func (a *Adapter) DetectElements(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsVisionConfig,
	splitWord bool,
) ([]*element.Element, error) {
	select {
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	cgRect := C.CGRect{
		origin: C.CGPoint{x: C.double(screenBounds.Min.X), y: C.double(screenBounds.Min.Y)},
		size:   C.CGSize{width: C.double(screenBounds.Dx()), height: C.double(screenBounds.Dy())},
	}

	var detectText C.int
	if cfg.DetectText {
		detectText = 1
	}

	var detectRectangles C.int
	if cfg.DetectRectangles {
		detectRectangles = 1
	}

	var wordLevel C.int
	if splitWord {
		wordLevel = 1
	}

	cCfg := C.NeruVisionConfig{
		detectText:             detectText,
		detectRectangles:       detectRectangles,
		requestTimeoutMS:       C.int(cfg.RequestTimeoutMS),
		rectangleMaxCandidates: C.int(cfg.RectangleMaxCandidates),
		rectangleMinSize:       C.double(cfg.RectangleMinSize),
		rectangleMinAspect:     C.double(cfg.RectangleMinAspect),
		rectangleMaxAspect:     C.double(cfg.RectangleMaxAspect),
		wordLevel:              wordLevel,
	}

	result := C.NeruDetectElements(cgRect, cCfg)
	if result == nil {
		return nil, nil
	}

	defer C.NeruFreeVisionResult(result)

	count := int(result.count)
	if count == 0 {
		return nil, nil
	}

	regions := make([]DetectedRegion, 0, count)
	cRegions := (*[1 << 30]C.VisionRegion)(unsafe.Pointer(result.regions))[:count:count]

	for _, cRegion := range cRegions {
		isText := cRegion.isText != 0
		if isText && !cfg.DetectText {
			continue
		}
		if !isText && !cfg.DetectRectangles {
			continue
		}
		if float64(cRegion.score) < cfg.MinimumConfidence {
			continue
		}

		region := DetectedRegion{
			Bounds: image.Rectangle{
				Min: image.Point{X: int(cRegion.x), Y: int(cRegion.y)},
				Max: image.Point{
					X: int(cRegion.x + cRegion.width),
					Y: int(cRegion.y + cRegion.height),
				},
			},
			Score:  float64(cRegion.score),
			IsText: isText,
		}
		if cRegion.label != nil {
			region.Label = C.GoString(cRegion.label)
		}
		regions = append(regions, region)
	}

	merged := MergeRegions(regions, cfg.MergeIOUThreshold)

	// Filter regions outside the window bounds
	windowElements := make([]DetectedRegion, 0, len(merged))
	for _, region := range merged {
		if region.Bounds.Overlaps(screenBounds) {
			windowElements = append(windowElements, region)
		}
	}

	classifier := newRegionClassifier(cfg)

	elements, skipped := elementsFromRegions(windowElements, &classifier)

	a.logger.Debug("Vision detection complete",
		zap.Int("raw_regions", count),
		zap.Int("merged_elements", len(elements)),
		zap.Int("skipped_regions", skipped),
	)

	return elements, nil
}

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

// Health checks whether Vision Framework is available.
func (a *Adapter) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Quick smoke test: attempt a screen capture
	_, err := a.CaptureScreen(ctx)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeInternal, "vision framework health check failed")
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

// readTextScale is how many pixels per point ReadText captures at, before
// NeruReadText scales a line to the heights it reads at. Of one, two and three,
// three misread the fewest lines of the OCR fixtures.
const readTextScale = 3

// readTextPadding widens a hint's box before ReadText captures it, since an
// OCR word box sits tight on the glyphs and the recognizer misses clipped
// edges.
const readTextPadding = 4

// readTextMu lets one read run at a time. A second one gives up at once rather
// than queue.
var readTextMu sync.Mutex

// ReadText captures rect at readTextScale and reads it. The recognizer cannot
// be canceled, so a read that outlives ctx keeps running on its own and its
// result is dropped. Holding readTextMu means that costs at most one read in
// flight.
func (a *Adapter) ReadText(ctx context.Context, rect image.Rectangle) (string, error) {
	if rect.Empty() {
		return "", derrors.New(derrors.CodeInvalidInput, "text reading needs a region with an area")
	}

	timeout, err := captureTimeoutMS(ctx)
	if err != nil {
		return "", err
	}

	if !readTextMu.TryLock() {
		return "", derrors.New(derrors.CodeActionFailed, "a text read is already running")
	}

	type result struct {
		text string
		err  error
	}

	done := make(chan result, 1)

	go func() {
		defer readTextMu.Unlock()

		text, err := readTextNow(rect.Inset(-readTextPadding), timeout)
		done <- result{text: text, err: err}
	}()

	select {
	case got := <-done:
		return got.text, got.err
	case <-ctx.Done():
		return "", derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "text reading timed out")
	}
}

// readTextNow captures region and reads it.
func readTextNow(region image.Rectangle, timeout C.int64_t) (string, error) {
	cgRect := C.CGRect{
		origin: C.CGPoint{x: C.double(region.Min.X), y: C.double(region.Min.Y)},
		size:   C.CGSize{width: C.double(region.Dx()), height: C.double(region.Dy())},
	}

	var captured C.CGRect

	cgImage := C.NeruCaptureRectAtScale(cgRect, readTextScale, &captured, timeout)
	if uintptr(cgImage) == 0 {
		return "", derrors.New(derrors.CodeActionFailed, "failed to capture the region to read")
	}
	defer C.CGImageRelease(cgImage)

	cText := C.NeruReadText(cgImage)
	if cText == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(cText))

	return C.GoString(cText), nil
}

// readLinesScale is how much larger than its size in points a region is
// captured for ReadLines, which is a Retina display's own resolution.
const readLinesScale = 2

// ReadLines captures region and reads it with the recognizer level names.
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

	timeout, err := captureTimeoutMS(ctx)
	if err != nil {
		return nil, err
	}

	started := time.Now()

	var lines [][]ports.TextPiece

	inHelper := level == ports.TextAccurate && a.linesHelper != ""
	if inHelper {
		lines, err = a.readLinesInHelper(ctx, region, timeout)
	} else {
		lines, err = readLinesHere(region, level, timeout)
	}

	if err != nil {
		return nil, err
	}

	a.logger.Debug("Read lines",
		zap.Bool("accurate", level == ports.TextAccurate),
		zap.Bool("helper", inHelper),
		zap.Int("line_count", len(lines)),
		zap.Duration("duration", time.Since(started)))

	return lines, nil
}

// readLinesHere captures region and reads it in this process.
func readLinesHere(
	region image.Rectangle,
	level ports.TextLevel,
	timeout C.int64_t,
) ([][]ports.TextPiece, error) {
	accurate := C.int(0)
	if level == ports.TextAccurate {
		accurate = 1
	}

	result := C.NeruReadLines(cRect(region), readLinesScale, accurate, timeout)
	if result == nil {
		return nil, derrors.New(derrors.CodeActionFailed, "failed to read the region's text")
	}
	defer C.NeruFreeTextResult(result)

	return groupLines(linePiecesOf(result)), nil
}

func cRect(r image.Rectangle) C.CGRect {
	return C.CGRect{
		origin: C.CGPoint{x: C.double(r.Min.X), y: C.double(r.Min.Y)},
		size:   C.CGSize{width: C.double(r.Dx()), height: C.double(r.Dy())},
	}
}
