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
	"unsafe"

	"go.uber.org/zap"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin" // ensure darwin CGo .m files are compiled
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
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

	// Convert screen bounds to CGRect
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

	// Convert C regions to Go DetectedRegions
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

	// Merge overlapping regions via NMS
	merged := MergeRegions(regions, cfg.MergeIOUThreshold)

	// Filter regions outside the window bounds
	windowElements := make([]DetectedRegion, 0, len(merged))
	for _, region := range merged {
		if region.Bounds.Overlaps(screenBounds) {
			windowElements = append(windowElements, region)
		}
	}

	// Classify and convert to domain elements
	classifier := newRegionClassifier(cfg)

	elements, skipped := elementsFromRegions(windowElements, &classifier)

	a.logger.Debug("Vision detection complete",
		zap.Int("raw_regions", count),
		zap.Int("merged_elements", len(elements)),
		zap.Int("skipped_regions", skipped),
	)

	return elements, nil
}

// CaptureScreen captures the current screen image for the primary display.
func (a *Adapter) CaptureScreen(_ context.Context) (*image.RGBA, error) {
	cgImage := C.NeruCaptureScreen()
	if uintptr(cgImage) == 0 {
		return nil, derrors.New(derrors.CodeInternal, "failed to capture screen")
	}

	defer C.CGImageRelease(cgImage)

	width := int(C.CGImageGetWidth(cgImage))
	height := int(C.CGImageGetHeight(cgImage))

	if width == 0 || height == 0 {
		return nil, derrors.New(derrors.CodeInternal, "captured screen image has zero size")
	}

	bytesPerRow := width * bytesPerPixel
	buf := make([]byte, height*bytesPerRow)

	colorSpace := C.CGColorSpaceCreateDeviceRGB()

	defer C.CGColorSpaceRelease(colorSpace)

	ctx := C.CGBitmapContextCreate(
		unsafe.Pointer(&buf[0]),
		C.size_t(width),
		C.size_t(height),
		8, // bits per component
		C.size_t(bytesPerRow),
		colorSpace,
		C.kCGImageAlphaPremultipliedLast,
	)

	if uintptr(ctx) == 0 {
		return nil, derrors.New(derrors.CodeInternal, "failed to create bitmap context")
	}
	defer C.CGContextRelease(ctx)

	// Draw the captured image into our context
	C.CGContextDrawImage(ctx, C.CGRect{
		origin: C.CGPoint{x: 0, y: 0},
		size:   C.CGSize{width: C.double(width), height: C.double(height)},
	}, cgImage)

	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, buf)

	return img, nil
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

// DetectWLKBPTR reports not-supported: wl-kbptr strategy is only implemented on Linux.
func (a *Adapter) DetectWLKBPTR(
	_ context.Context,
	_ image.Rectangle,
) ([]*element.Element, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"the wl-kbptr strategy is only implemented on Linux",
	)
}
