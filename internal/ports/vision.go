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

	// CaptureScreen returns the current screen image. Which screen "current"
	// means is the platform's own answer: macOS captures the primary display,
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
}
