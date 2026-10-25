package vision

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.VisionPort. Its methods live in the platform files:
// the macOS Vision framework in adapter_darwin.go, tesseract in
// adapter_linux.go, Windows.Media.Ocr in adapter_windows.go, and
// CodeNotSupported stubs in adapter_other.go.
//
// The adapter captures the screen and runs text recognition and rectangle
// detection (plus saliency on macOS) to detect interactive UI elements. A
// heuristic classifier assigns the results roles and confidence scores.
type Adapter struct {
	logger *zap.Logger
}

// NewAdapter creates a vision adapter.
func NewAdapter(logger *zap.Logger) ports.VisionPort {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{
		logger: logger.Named("vision"),
	}
}

// Ensure Adapter implements ports.VisionPort. Every platform file satisfies it,
// the `other` slot with CodeNotSupported stubs (adapter_other.go), so this
// assertion holds on every target and catches a signature drift in any of them.
var _ ports.VisionPort = (*Adapter)(nil)
