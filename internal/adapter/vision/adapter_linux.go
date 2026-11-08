//go:build linux

package vision

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/image/draw"

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
// KDE) plus tesseract (platform/linux/ocr.c) for text and the pure-Go contour
// detector for rectangles, run concurrently over one frame (rectangles.go).
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

	// Check the engine before capturing, or a machine with no language data
	// would have its window captured on every activation for nothing. After the
	// first call this is a cached handle.
	if cfg.DetectText {
		engineErr := platformlinux.OCRHealth()
		if engineErr != nil {
			return nil, engineErr
		}
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	// Canceling on return stops the rectangle pass when the text half fails.
	rectCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rectangles := a.startRectangles(rectCtx, img, region, cfg)

	var (
		words []platformlinux.OCRWord
		stats platformlinux.OCRStats
		text  []DetectedRegion
	)

	if cfg.DetectText {
		words, stats, err = platformlinux.RecognizeText(img, platformlinux.OCRParams{
			WordLevel: splitWord,
			TimeoutMS: cfg.RequestTimeoutMS,
		})
		if err != nil {
			// A refusal already names what to install, so it passes unchanged.
			if derrors.IsNotSupported(err) {
				return nil, err
			}

			// The cost and budget tell an early give-up from a timeout. The error
			// never carries the recognized text.
			return nil, derrors.Wrapf(
				err,
				derrors.GetCode(err),
				"text recognition failed after %s (budget %d ms, frame %dx%d)",
				stats.Recognition,
				cfg.RequestTimeoutMS,
				img.Rect.Dx(),
				img.Rect.Dy(),
			)
		}

		text = regionsFromWords(toRecognizedWords(words), region, img.Rect, cfg.MinimumConfidence)
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
		zap.Duration("recognition", stats.Recognition),
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

// toRecognizedWords adapts the platform bridge's word type to the shared one,
// so the region mapping stays testable on a non-Linux host.
func toRecognizedWords(words []platformlinux.OCRWord) []recognizedWord {
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

	scale := 1.0
	if img.Rect.Dy() > 0 {
		scale = float64(img.Rect.Dy()) / float64(region.Dy())
	}

	return detectionFrame{img: img, origin: region.Min, region: region, scale: scale}, nil
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
// The recognition half catches missing tesseract language data, a separate
// distribution package, and the error names the file. Checking it warms the
// engine, whose handle is cached. Nothing in the daemon calls this today, and
// DetectElements runs the same checks.
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

	return platformlinux.OCRHealth()
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

// ReadText reports CodeNotSupported. tesseract already reads text accurately
// in the detection pass, so copy_text keeps the text the hint has.
func (a *Adapter) ReadText(_ context.Context, _ image.Rectangle) (string, error) {
	return "", derrors.New(derrors.CodeNotSupported, "accurate re-reading is not needed on Linux")
}

// ReadLines captures region and has tesseract read it a character at a time.
// An accurate read also reads it in the languages of other scripts the locale
// names, on an engine of its own, as macOS reads in the preferred languages,
// since eng reads them as Latin noise. The fast read, which a keystroke waits
// on, stays eng alone.
func (a *Adapter) ReadLines(
	ctx context.Context,
	region image.Rectangle,
	level ports.TextLevel,
) ([][]ports.TextPiece, error) {
	if region.Empty() {
		err := platformlinux.LinesReadable()
		if err != nil {
			return nil, err
		}

		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"reading lines needs a region with an area",
		)
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	timeoutMS := 0
	if deadline, ok := ctx.Deadline(); ok {
		timeoutMS = max(int(time.Until(deadline).Milliseconds()), 1)
	}

	params := platformlinux.OCRParams{TimeoutMS: timeoutMS}

	lines, stats, err := readLinePieces(img, img, region, params)
	if err != nil {
		if derrors.IsNotSupported(err) {
			return nil, err
		}

		return nil, derrors.Wrapf(
			err,
			derrors.GetCode(err),
			"reading lines failed after %s (frame %dx%d)",
			stats.Recognition,
			img.Rect.Dx(),
			img.Rect.Dy(),
		)
	}

	second := ""
	if level == ports.TextAccurate {
		second = a.secondLanguage()
	}

	if second != "" {
		params.Language = second
		if timeoutMS > 0 {
			params.TimeoutMS = max(timeoutMS-int(stats.Recognition.Milliseconds()), 1)
		}

		secondLines, secondStats, secondErr := readLinePieces(
			atLeastTwoPixelsPerPoint(img, region), img, region, params,
		)
		if secondErr != nil {
			a.logger.Debug("Second-language read failed, keeping the eng read",
				zap.Duration("recognition", secondStats.Recognition), zap.Error(secondErr))
		} else {
			lines = mergeSecondLines(lines, secondLines)
			stats.Recognition += secondStats.Recognition
		}
	}

	a.logger.Debug("Read lines",
		zap.Bool("accurate", level == ports.TextAccurate),
		zap.Bool("second_language", second != ""),
		zap.Int("line_count", len(lines)),
		zap.Duration("recognition", stats.Recognition))

	return lines, nil
}

// secondLanguageWarned keeps missing language data to one warning.
var secondLanguageWarned sync.Once

// secondLanguage is the languages an accurate read reads a second time in, or
// "" for none. A locale naming one whose data is missing warns once and reads
// in the rest.
func (a *Adapter) secondLanguage() string {
	language, err := platformlinux.SecondLanguage()
	if err != nil {
		secondLanguageWarned.Do(func() {
			a.logger.Warn("Vim mode cannot read every language the locale names", zap.Error(err))
		})
	}

	return language
}

// wideReadPixelsPerPoint is how many pixels per point the second-language read
// gets. At one pixel per point, small Chinese text reads better as eng's Latin
// noise, so tesseract keeps that reading, while at two it reads the
// characters.
const wideReadPixelsPerPoint = 2

// atLeastTwoPixelsPerPoint is img, which shows region, scaled up to
// wideReadPixelsPerPoint pixels per point when it has fewer.
func atLeastTwoPixelsPerPoint(img *image.RGBA, region image.Rectangle) *image.RGBA {
	if region.Dx() <= 0 || img.Rect.Dx() >= wideReadPixelsPerPoint*region.Dx() {
		return img
	}

	scaled := image.NewRGBA(image.Rect(
		0, 0, wideReadPixelsPerPoint*region.Dx(), wideReadPixelsPerPoint*region.Dy(),
	))
	draw.CatmullRom.Scale(scaled, scaled.Rect, img, img.Rect, draw.Src, nil)

	return scaled
}

// readLinePieces has tesseract read read, a frame of img at any scale, and lays
// the characters out as ReadLines pieces over img, which shows region. The
// layout cuts at img's own pixels, since scaling blurs away the one-pixel gaps
// between small characters.
func readLinePieces(
	read, img *image.RGBA,
	region image.Rectangle,
	params platformlinux.OCRParams,
) ([][]ports.TextPiece, platformlinux.OCRStats, error) {
	found, stats, err := platformlinux.RecognizeLines(read, params)
	if err != nil {
		return nil, stats, err
	}

	symbols := make([]lineSymbol, 0, len(found))
	for _, symbol := range found {
		symbols = append(symbols, lineSymbol{
			text: symbol.Text,
			ink:  toRegion(symbol.Bounds.Add(read.Rect.Min), read.Rect, img.Rect),
			line: symbol.Line,
			word: symbol.Word,
		})
	}

	return piecesFromSymbols(symbols, img, region), stats, nil
}
