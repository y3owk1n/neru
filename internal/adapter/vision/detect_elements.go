//go:build darwin || linux

package vision

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// macOS and Linux answer DetectElements alike. The platform captures the
// frame (captureText), yomi reads its text (yomi_read.go), and the pure-Go
// contour detector finds button shapes in the same frame (rectangles.go), the
// two running at once.
//
// Privacy: recognized text is screen content. It reaches an element's title
// and search text and nowhere else. Nothing here logs it or counts it into a
// message. Debug lines carry durations and counts.

// DetectElements captures screenBounds and returns the text and button shapes
// found in it as hintable elements.
//
// screenBounds, normally the focused window in global top-left-origin unscaled
// pixels, is honored rather than widened, since full-display OCR takes
// seconds. It also places the results, so an empty rectangle is refused
// rather than read as "the whole screen".
func (a *Adapter) DetectElements(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsVisionConfig,
	splitWord bool,
) ([]*element.Element, error) {
	region, err := detectionRegion(ctx, screenBounds, "vision")
	if err != nil {
		return nil, err
	}

	if !cfg.DetectText && !cfg.DetectRectangles {
		// Skip the capture when nothing would read the frame.
		a.logger.Debug("Vision detection skipped: text and rectangle detection are disabled")

		return nil, nil
	}

	img, region, err := a.captureText(ctx, region)
	if err != nil {
		return nil, err
	}

	// Canceling on return stops the rectangle pass when the text half fails.
	rectCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rectangles := a.startRectangles(rectCtx, img, region, cfg)

	var (
		words       []recognizedWord
		recognition time.Duration
		text        []DetectedRegion
	)

	if cfg.DetectText {
		readCtx := ctx

		if cfg.RequestTimeoutMS > 0 {
			var cancelRead context.CancelFunc

			readCtx, cancelRead = context.WithTimeout(
				ctx, time.Duration(cfg.RequestTimeoutMS)*time.Millisecond,
			)
			defer cancelRead()
		}

		started := time.Now()
		lines, readErr := readYomi(readCtx, img, region)
		recognition = time.Since(started)

		if readErr != nil {
			// The cost and budget tell an early give-up from a timeout. The error
			// never carries the recognized text.
			return nil, derrors.Wrapf(
				readErr,
				derrors.GetCode(readErr),
				"text recognition failed after %s (budget %d ms, frame %dx%d)",
				recognition,
				cfg.RequestTimeoutMS,
				img.Rect.Dx(),
				img.Rect.Dy(),
			)
		}

		words = yomiWords(lines, splitWord)
		text = regionsFromWords(words, region, img.Rect, cfg.MinimumConfidence)
	}

	found, err := rectangles()
	if err != nil && !cfg.DetectText {
		return nil, err
	}

	rects := withoutTextOverlap(found, text, cfg.MergeIOUThreshold)
	merged := MergeRegions(append(text, rects...), cfg.MergeIOUThreshold)

	classifier := newRegionClassifier(cfg)

	elements, skipped := elementsFromRegions(merged, &classifier)

	a.logger.Debug("Vision detection complete",
		zap.Duration("recognition", recognition),
		zap.Int("frame_width", img.Rect.Dx()),
		zap.Int("frame_height", img.Rect.Dy()),
		zap.Int("raw_words", len(words)),
		zap.Int("rectangles", len(rects)),
		zap.Int("merged_elements", len(elements)),
		zap.Int("skipped_regions", skipped),
		zap.Bool("word_level", splitWord),
	)

	return elements, nil
}
