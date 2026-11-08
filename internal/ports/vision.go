package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// VisionPort is the interface for vision-based element detection using
// macOS Vision Framework (or platform equivalents). Implementations capture
// screenshots and detect UI elements via text recognition, rectangle detection,
// and saliency analysis.
//
// Only used when hints.strategy is set to "vision" for the frontmost window.
// System-level components (menubar, dock, notification center, etc.) always
// use the AX tree regardless of strategy.
type VisionPort interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(ctx context.Context) error

	// DetectElements captures a screenshot of the frontmost window and returns
	// detected interactive elements. The screenBounds parameter constrains
	// detection to the window region. Implementations use Vision Framework
	// requests (text recognition, rectangle detection, saliency) and a
	// heuristic classifier to assign element roles.
	DetectElements(
		ctx context.Context,
		screenBounds image.Rectangle,
		cfg config.HintsVisionConfig,
		splitWord bool,
	) ([]*element.Element, error)

	// DetectContours captures screenBounds and returns the interactive targets
	// the contour detector (ported from wl-kbptr) finds in it, as clickable
	// vision-only buttons with no title. cfg tunes the detector. The caller
	// bounds the pass with ctx.
	DetectContours(
		ctx context.Context,
		screenBounds image.Rectangle,
		cfg config.HintsContourConfig,
	) ([]*element.Element, error)

	// DetectSegments captures screenBounds and returns the targets the segment
	// detector finds in it (controls, pictures and words), as clickable
	// vision-only buttons with no title. cfg says what to hint. The caller
	// bounds the pass with ctx.
	DetectSegments(
		ctx context.Context,
		screenBounds image.Rectangle,
		cfg config.HintsSegmentConfig,
	) ([]*element.Element, error)

	// DetectSegmentsAsRead runs the segment detector as DetectSegments does,
	// on the frame the last fast read of region captured when there is one,
	// so it looks at the screen that read looked at and captures nothing. A
	// Linux capture takes the overlay off the screen, so this is what keeps a
	// read's icon pass from blinking it. A platform whose read keeps a frame the
	// detector cannot take captures afresh. So does a region with no read.
	DetectSegmentsAsRead(
		ctx context.Context,
		region image.Rectangle,
		cfg config.HintsSegmentConfig,
	) ([]*element.Element, error)

	// CaptureScreen returns the current screen image. Which screen "current"
	// means is the platform's own answer. macOS captures the primary display,
	// Linux the screen holding the cursor, because Wayland exposes no primary
	// display at all. Either way the image's bounds start at (0, 0) and carry no
	// global origin.
	//
	// Used by DetectElements internally, but exposed for testing or inspection.
	//
	// The result is arbitrary screen content. Implementations and callers must
	// never log it, derive log text from it, write it to disk, or keep it past
	// the detection that asked for it.
	CaptureScreen(ctx context.Context) (*image.RGBA, error)

	// ReadText captures rect, a screen region in global coordinates, and
	// returns the text in it, read from that region alone rather than from the
	// whole frame DetectElements reads. It is a second look at one hint, for
	// copy_text. A platform whose DetectElements already reads text accurately
	// returns CodeNotSupported, and the caller keeps the text it has. The
	// caller bounds the read with ctx. Like CaptureScreen's frame, the text is
	// screen content and must never be logged.
	ReadText(ctx context.Context, rect image.Rectangle) (string, error)

	// ReadLines captures region, in global coordinates, and returns its text
	// a line at a time, top to bottom, each line's whitespace-separated words
	// with their bounds. An accurate read spells small text right where a
	// fast one misreads it, and its first use loads a model that can take
	// seconds, during which other reads in the process wait. The caller
	// bounds the read with ctx. The text is screen content and must never be
	// logged. Platforms without it report CodeNotSupported, for any region.
	// An empty region captures nothing, so it asks whether lines can be read
	// at all: CodeNotSupported where they cannot, CodeInvalidInput where they
	// can.
	ReadLines(ctx context.Context, region image.Rectangle, level TextLevel) ([][]TextPiece, error)

	// ReadLinesOnceChanged reads region as ReadLines does at TextFast, once
	// it has changed from what the last fast read of it captured and has
	// stopped changing, as after a scroll the app may still be animating.
	// One that does not change within a bound, as a scroll at the end of a
	// page, gives the last read's lines again without reading. Canceling ctx
	// stops it.
	ReadLinesOnceChanged(ctx context.Context, region image.Rectangle) ([][]TextPiece, error)

	// AccurateReadDiffers reports whether ReadLines at TextAccurate reads
	// anything TextFast does not. Where it does not, a caller holding a fast
	// read keeps it rather than reading the same thing twice. The accurate
	// read is there for accuracy, not for a newer frame, so keeping the fast
	// one loses nothing.
	AccurateReadDiffers() bool

	// CaptureExcludesOwnWindows reports whether a capture leaves Neru's own
	// windows out of the frame. When it does not, a hints refresh that reads
	// the screen takes the old labels off first, or it reads them back as
	// targets.
	CaptureExcludesOwnWindows() bool
}

// TextLevel is how carefully ReadLines reads.
type TextLevel int

const (
	// TextFast reads quickly and misreads small text.
	TextFast TextLevel = iota
	// TextAccurate reads slowly and gets small text right.
	TextAccurate
)

// TextPiece is a word of a line read off the screen and the global bounds it
// was read from. Word numbers the words of its line.
type TextPiece struct {
	Text   string
	Bounds image.Rectangle
	Word   int
	// Background is the color behind its word, as 0xRRGGBB, or zero when
	// the platform does not say.
	Background uint32
}
