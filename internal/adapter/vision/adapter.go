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
	// linesHelper is the hidden command this executable reads text lines
	// accurately with in a process of its own, or empty to read them here.
	linesHelper string
}

// Option configures an Adapter.
type Option func(*Adapter)

// WithLinesHelper has accurate line reads run this executable with the hidden
// command, in a process of their own, so a model compiling for a new script
// holds up no read in this one. lines_helper_darwin.go says why.
func WithLinesHelper(command string) Option {
	return func(a *Adapter) { a.linesHelper = command }
}

// NewAdapter creates a vision adapter.
func NewAdapter(logger *zap.Logger, opts ...Option) ports.VisionPort {
	if logger == nil {
		logger = zap.NewNop()
	}

	adapter := &Adapter{logger: logger.Named("vision")}
	for _, opt := range opts {
		opt(adapter)
	}

	return adapter
}

// Ensure Adapter implements ports.VisionPort. Every platform file satisfies it,
// the `other` slot with CodeNotSupported stubs (adapter_other.go), so this
// assertion holds on every target and catches a signature drift in any of them.
var _ ports.VisionPort = (*Adapter)(nil)
