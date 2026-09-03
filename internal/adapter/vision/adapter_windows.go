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

// Windows answers this port with two native pieces: screen capture through GDI
// (BitBlt off the desktop DC, platform/windows/screencapture.go) and text
// recognition through Windows.Media.Ocr (platform/windows/ocr.go), the WinRT
// engine every Windows 10 and 11 desktop ships.
//
// Like Linux it answers the *text* half of the strategy only. macOS runs three
// Vision requests — text recognition, rectangle detection and saliency — and
// an OCR engine answers the first, so hints.vision.detect_rectangles and its
// four rectangle_* companions stay declared macOS-only
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md). The engine
// reports no per-word confidence either, so the three confidence floors are
// declared inert here and every word carries a score of one.
//
// Privacy runs through every line below. Recognized text is screen content: it
// reaches an element's title and search text, where the hint pipeline needs it,
// and nowhere else. Nothing here logs it, counts characters of it into a
// message, or writes it anywhere; the debug lines carry durations and counts.

// DetectElements captures screenBounds and returns the text found in it as
// hintable elements.
//
// screenBounds is where the caller wants hints — normally the focused window —
// in global top-left-origin unscaled pixels, and it is honored rather than
// widened: it is what places the results, so an empty rectangle is refused
// rather than read as "the whole screen".
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

	region := screenBounds.Canon()
	if region.Empty() {
		return nil, derrors.Newf(
			derrors.CodeActionFailed,
			"vision detection needs a region to read; %v is empty",
			screenBounds,
		)
	}

	if !cfg.DetectText {
		// Text is the whole of what this backend detects, so with
		// hints.vision.detect_text off there is nothing left to run.
		a.logger.Debug("Vision detection skipped: text detection is disabled")

		return nil, nil
	}

	// The engine is checked before the screen is read: a machine with no OCR
	// language pack would otherwise have its focused window captured on every
	// activation only to be told there is nothing to read it with. After the
	// first call this is a cached handle.
	engineErr := platformwindows.OCRHealth()
	if engineErr != nil {
		return nil, engineErr
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	words, stats, err := platformwindows.RecognizeText(img, platformwindows.OCRParams{
		WordLevel: splitWord,
		TimeoutMS: cfg.RequestTimeoutMS,
	})
	if err != nil {
		// Dimensions and durations describe the work, never its content.
		a.logger.Error("Vision detection failed",
			zap.Duration("recognition", stats.Recognition),
			zap.Int("budget_ms", cfg.RequestTimeoutMS),
			zap.Int("frame_width", img.Rect.Dx()),
			zap.Int("frame_height", img.Rect.Dy()),
			zap.Error(err),
		)

		return nil, err
	}

	regions := regionsFromWords(
		toRecognizedWords(words),
		region,
		img.Rect,
		cfg.MinimumConfidence,
	)

	merged := MergeRegions(regions, cfg.MergeIOUThreshold)

	classifier := newRegionClassifier(cfg)

	elements, skipped := elementsFromRegions(merged, &classifier)

	a.logger.Debug("Vision detection complete",
		zap.Duration("recognition", stats.Recognition),
		zap.Int("frame_width", img.Rect.Dx()),
		zap.Int("frame_height", img.Rect.Dy()),
		zap.Int("raw_words", len(words)),
		zap.Int("merged_elements", len(elements)),
		zap.Int("skipped_regions", skipped),
		zap.Bool("word_level", splitWord),
	)

	return elements, nil
}

// toRecognizedWords adapts the platform bridge's word type to the shared one
// the region mapping is written against.
func toRecognizedWords(words []platformwindows.OCRWord) []recognizedWord {
	converted := make([]recognizedWord, 0, len(words))

	for _, word := range words {
		converted = append(converted, recognizedWord{
			Text:       word.Text,
			Bounds:     word.Bounds,
			Confidence: word.Confidence,
		})
	}

	return converted
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

// Health reports whether the vision strategy can run on this machine.
//
// Capture needs no probe: every interactive Windows desktop can read its own
// pixels, and a session without one fails at the capture rather than here.
// The recognition half catches the failure a user can fix: the OCR language
// pack is installed per language, separately from Windows itself, so a
// machine that runs Neru can still have none. The error names the remedy.
// Asking also warms the engine; the handle is kept for the process.
func (a *Adapter) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	return platformwindows.OCRHealth()
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
