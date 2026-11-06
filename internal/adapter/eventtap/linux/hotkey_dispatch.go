//go:build linux

package linux

import "sync"

// HotkeyDispatcher runs hotkey callbacks off the key-reading goroutine, one at
// a time, in queue order.
//
// Off it because a callback takes the mode handler's lock, and the handler
// waits on the reader while holding it (the proxy's session ack, the X11
// loop's exit). In order because a release running after the next press
// would cancel the repeat that press started.
//
// The queue is unbounded, because a dropped release would leave a repeat
// outliving its key. It grows by two callbacks per hotkey press, so even a
// stuck handler adds only a few entries per hold.
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
