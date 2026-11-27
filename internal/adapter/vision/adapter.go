package vision

import (
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.VisionPort. Its methods live in the platform files:
// each platform's capture in adapter_darwin.go, adapter_linux.go and
// adapter_windows.go, the reading every platform shares in detect_elements.go
// and yomi_read.go, and CodeNotSupported stubs in adapter_other.go.
//
// The adapter captures the screen and runs text recognition and rectangle
// detection to detect interactive UI elements. A
// heuristic classifier assigns the results roles and confidence scores.
type Adapter struct {
	logger *zap.Logger
	// lastRead is the last fast line read, which ReadLinesOnceChanged waits
	// for the screen to change from.
	lastRead lastRead
	// accurateReady is the channel PrepareAccurateRead returns, set by its
	// first call.
	accurateReady atomic.Pointer[chan struct{}]
}

// NewAdapter creates a vision adapter.
func NewAdapter(logger *zap.Logger) ports.VisionPort {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Adapter{logger: logger.Named("vision")}
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
// fails leaves the first accurate read to do that work itself.
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
