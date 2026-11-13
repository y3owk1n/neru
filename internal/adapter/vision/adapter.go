package vision

import (
	"sync/atomic"
	"time"

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
	// lastRead is the last fast line read, which ReadLinesOnceChanged waits
	// for the screen to change from.
	lastRead lastRead
	// accurateReady is the channel PrepareAccurateRead returns, set by its
	// first call.
	accurateReady atomic.Pointer[chan struct{}]
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

// ForgetLastRead implements ports.VisionPort.
func (a *Adapter) ForgetLastRead() { a.lastRead.forget() }

// PrepareAccurateRead implements ports.VisionPort. The first call starts the
// platform's preparation (accurateReadPreparation) on a goroutine of its own,
// and every call returns the channel it closes. A platform with nothing to
// prepare gets a closed channel back from the first call on.
func (a *Adapter) PrepareAccurateRead() <-chan struct{} {
	if ready := a.accurateReady.Load(); ready != nil {
		return *ready
	}

	prepare := a.accurateReadPreparation()

	ready := make(chan struct{})
	if prepare == nil {
		close(ready)
	}

	if !a.accurateReady.CompareAndSwap(nil, &ready) {
		return *a.accurateReady.Load()
	}

	if prepare != nil {
		go a.prepareAccurateRead(ready, prepare)
	}

	return ready
}

// prepareAccurateRead runs prepare and closes ready. A preparation that
// fails leaves the first accurate read to compile what it needs, as it would
// with none.
func (a *Adapter) prepareAccurateRead(ready chan struct{}, prepare func() error) {
	defer close(ready)

	started := time.Now()

	err := prepare()
	if err != nil {
		a.logger.Warn("Failed to prepare text recognition", zap.Error(err))

		return
	}

	a.logger.Debug("Prepared text recognition", zap.Duration("duration", time.Since(started)))
}
