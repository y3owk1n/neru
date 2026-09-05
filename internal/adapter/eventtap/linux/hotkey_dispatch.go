//go:build linux

package linux

import "sync"

// HotkeyDispatcher runs hotkey callbacks off the goroutine that reads key
// events, one at a time, in the order they were queued.
//
// Off that goroutine because a callback takes the mode handler's lock, and the
// handler waits on the reader while holding it (the proxy's session ack, the
// X11 loop's exit), so running it inline could deadlock. In order because a
// goroutine per callback has none: a release and the press that follows it
// could run swapped, and the release would then cancel the repeat the new
// press had just started, leaving a held key that stops repeating.
//
// The queue is unbounded rather than dropping under pressure, because a
// release is the one thing that ends a held key's repeat: dropped, the repeat
// would outlive the key. What queues is two callbacks per hotkey press, so
// even a handler stuck for the length of a hold grows it by a few entries.
type HotkeyDispatcher struct {
	mu      sync.Mutex
	ready   *sync.Cond
	queue   []func()
	stopped bool
}

// NewHotkeyDispatcher starts a dispatcher.
func NewHotkeyDispatcher() *HotkeyDispatcher {
	dispatcher := &HotkeyDispatcher{}
	dispatcher.ready = sync.NewCond(&dispatcher.mu)

	go dispatcher.run()

	return dispatcher
}

// Dispatch queues callback. It never blocks the reader.
func (d *HotkeyDispatcher) Dispatch(callback func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	d.queue = append(d.queue, callback)
	d.ready.Signal()
}

// Stop ends the dispatcher once it has run what is queued. It does not wait,
// because a queued callback may be waiting on a lock the owner holds.
func (d *HotkeyDispatcher) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	d.ready.Signal()
}

func (d *HotkeyDispatcher) run() {
	for {
		d.mu.Lock()

		for len(d.queue) == 0 && !d.stopped {
			d.ready.Wait()
		}

		if len(d.queue) == 0 {
			d.mu.Unlock()

			return
		}

		callback := d.queue[0]
		d.queue[0] = nil
		d.queue = d.queue[1:]
		d.mu.Unlock()

		callback()
	}
}
