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
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// Linux answers this port with two native pieces: screen capture
// (wlr-screencopy on wlroots compositors, XGetImage on X11, a PipeWire stream
// off the portal's ScreenCast session on KDE) and tesseract through
// platform/linux/ocr.c.
//
// It answers the *text* half of the strategy only. macOS runs three Vision
// requests — text recognition, rectangle detection and saliency — and an OCR
// engine answers the first. hints.vision.detect_rectangles and its four
// rectangle_* companions are declared macOS-only rather than met with a
// contour-detection library, so Linux `vision` is text-only and says so
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
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
// widened: full-display OCR takes seconds, one window is tens of milliseconds
// to a few hundred. It is also what places the results, so an empty rectangle
// is refused rather than read as "the whole screen": the frame would come back
// with no way to say where its top-left was.
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
		// hints.vision.detect_text off there is nothing left to run. Capturing
		// the screen to find nothing in it would be a plain waste.
		a.logger.Debug("Vision detection skipped: text detection is disabled")

		return nil, nil
	}

	// The engine is checked before the screen is read, not after. A machine
	// with no language data would otherwise have its focused window captured on
	// every activation only to be told there is nothing to read it with —
	// paying for a frame, and taking one, to answer a question about which
	// packages are installed. After the first call this is a cached handle.
	engineErr := platformlinux.OCRHealth()
	if engineErr != nil {
		return nil, engineErr
	}

	img, err := a.captureRegion(ctx, region)
	if err != nil {
		return nil, err
	}

	words, stats, err := platformlinux.RecognizeText(img, platformlinux.OCRParams{
		WordLevel: splitWord,
		TimeoutMS: cfg.RequestTimeoutMS,
	})
	if err != nil {
		// A failed recognition is logged with what it cost and what it was
		// given, because those are the two numbers that say which failure it
		// was: a frame the engine gave up on immediately reads nothing like one
		// that ran to the budget, and the budget is the option a user can turn.
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
// the region mapping is written against, so that mapping stays testable on a
// host with no Linux build.
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

// CaptureScreen returns the pixels currently on the active screen.
func (a *Adapter) CaptureScreen(ctx context.Context) (*image.RGBA, error) {
	return a.captureRegion(ctx, image.Rectangle{})
}

// DetectContours captures screenBounds and runs the contour detector over it.
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

	scale := 1.0
	if region.Dy() > 0 && img.Rect.Dy() > 0 {
		scale = float64(img.Rect.Dy()) / float64(region.Dy())
	}

	rects, err := contour.Detect(img, scale)
	if err != nil {
		return nil, err
	}

	return contour.Elements(region.Min, region, rects), nil
}

// Health reports whether the vision strategy can run on this machine, by
// checking both halves without exercising either destructively.
//
// The capture half is answered from the live backend label rather than by
// taking a frame: a health check that read the user's screen to find out which
// display server is running would be the wrong trade twice over. That catches
// the sessions whose label settles it — no display server at all, GNOME, an
// unrecognized Wayland compositor — and it deliberately does not catch a KDE
// session whose screen-sharing consent has not been approved yet. That is a
// permission rather than a capability, and it is what
// SystemPort.CheckScreenCapturePermission answers, ahead of the activation, so
// the user is asked instead of told the strategy is unavailable.
//
// The recognition half catches the failure a user can actually fix: tesseract
// language data is a separate distribution package from the library Neru links,
// so a machine that builds and starts Neru can still have no eng.traineddata on
// it. The error names the file.
//
// Initializing tesseract loads an LSTM model from disk, so a caller that does
// ask is also warming the engine — the handle is cached for the process, and a
// later detection finds it loaded. Nothing in the daemon calls this today; the
// same two checks run inside DetectElements, which is where a user meets them.
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

// backend names the live display server, which is what decides whether capture
// is possible at all. platform.DetectLinuxBackend is the one place that answer
// comes from (internal/adapter/platform/AGENTS.md).
func (a *Adapter) backend() string {
	return platform.DetectLinuxBackend().String()
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
