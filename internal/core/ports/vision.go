package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

// VisionPort defines the interface for vision-based element detection using
// macOS Vision Framework (or platform equivalents). Implementations capture
// screenshots and detect UI elements via text recognition, rectangle detection,
// and saliency analysis.
//
// Only used when hints.strategy is set to "vision" for the frontmost window.
// System-level components (menubar, dock, notification center, etc.) always
// use the AX tree regardless of strategy.
type VisionPort interface {
	HealthCheck

	// DetectElements captures a screenshot of the frontmost window and returns
	// detected interactive elements. The screenBounds parameter constrains
	// detection to the window region. Implementations use Vision Framework
	// requests (text recognition, rectangle detection, saliency) and a
	// heuristic classifier to assign element roles.
	DetectElements(ctx context.Context, screenBounds image.Rectangle) ([]*element.Element, error)

	// CaptureScreen returns the current screen image for the primary display.
	// Used by DetectElements internally, but exposed for testing or inspection.
	CaptureScreen(ctx context.Context) (*image.RGBA, error)
}
