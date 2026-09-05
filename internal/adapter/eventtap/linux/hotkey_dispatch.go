//go:build linux

package linux

import "go.uber.org/zap"

// hotkeyDispatchBufferSize bounds how many callbacks can wait on the
// dispatcher. Two per hotkey is the most a hold produces; the rest is slack for
// a handler that is briefly busy.
const hotkeyDispatchBufferSize = 64

// HotkeyDispatcher runs hotkey callbacks off the goroutine that reads key
// events, one at a time, in the order they were queued.
//
// Off that goroutine because a callback takes the mode handler's lock, and the
// handler waits on the reader while holding it (the proxy's session ack, the
// X11 loop's exit), so running it inline could deadlock. In order because a
// goroutine per callback has none: a release and the press that follows it
// could run swapped, and the release would then cancel the repeat the new
// press had just started, leaving a held key that stops repeating.
type HotkeyDispatcher struct {
	logger *zap.Logger
	queue  chan func()
}

// NewHotkeyDispatcher starts a dispatcher.
func NewHotkeyDispatcher(logger *zap.Logger) *HotkeyDispatcher {
	if logger == nil {
		logger = zap.NewNop()
	}

	dispatcher := &HotkeyDispatcher{
		logger: logger,
		queue:  make(chan func(), hotkeyDispatchBufferSize),
	}

	go dispatcher.run()

	return dispatcher
}

// Dispatch queues callback. It never blocks the reader: a full queue means the
// handler is wedged, and the callback is dropped with a warning, as the taps
// drop keys.
func (d *HotkeyDispatcher) Dispatch(callback func()) {
	select {
	case d.queue <- callback:
	default:
		d.logger.Warn("Hotkey dispatch queue full, dropping callback")
	}
}

// Stop ends the dispatcher once it has run what is queued. Only the reader
// queues, so the owner calls this after the reader has exited; it does not
// wait, because a queued callback may be waiting on a lock the owner holds.
func (d *HotkeyDispatcher) Stop() {
	close(d.queue)
}

func (d *HotkeyDispatcher) run() {
	for callback := range d.queue {
		callback()
	}
}
