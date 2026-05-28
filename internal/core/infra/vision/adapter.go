// Package vision provides vision-based element detection using the
// macOS Vision Framework. It implements ports.VisionPort with a
// heuristic classifier for role assignment.
package vision

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// Adapter implements ports.VisionPort using the macOS Vision Framework.
// On non-darwin platforms the implementation is a no-op stub.
//
// The adapter captures screenshots of the frontmost window and runs
// Vision Framework requests (text recognition, rectangle detection,
// saliency) to detect interactive UI elements. The results are passed
// through a heuristic classifier to assign roles and confidence scores.
type Adapter struct {
	logger *zap.Logger
}

// NewAdapter creates a new Vision Framework adapter.
func NewAdapter(logger *zap.Logger) ports.VisionPort {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{
		logger: logger.Named("infra.vision"),
	}
}
