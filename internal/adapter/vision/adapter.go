package vision

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
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
		logger: logger.Named("vision"),
	}
}

// Ensure Adapter implements ports.VisionPort. The non-darwin build satisfies it
// with CodeNotSupported stubs (adapter_other.go), so this assertion holds on
// every target and catches a signature drift in either half.
var _ ports.VisionPort = (*Adapter)(nil)
