package textinput

import (
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// callbackQueue runs a native field's callbacks in order on a goroutine of its
// own. The handler stops the session from inside a callback, and stopping
// waits for the field, so a callback must never run on the field's own
// thread. Nothing is dropped except a query that a newer one replaces, since
// only the latest text matters, and nothing runs once stop is called.
type callbackQueue struct {
	callbacks ports.TextInputCallbacks

	mu       sync.Mutex
	events   []queuedCallback
	wake     chan struct{}
	stopping chan struct{}
	stopOnce sync.Once
}

type queuedCallback struct {
	query   string
	changed bool
	confirm bool
	cancel  bool
}

func newCallbackQueue(callbacks ports.TextInputCallbacks) *callbackQueue {
	queue := &callbackQueue{
		callbacks: callbacks,
		wake:      make(chan struct{}, 1),
		stopping:  make(chan struct{}),
	}

	go queue.run()

	return queue
}

func (q *callbackQueue) queryChanged(query string) {
	q.post(queuedCallback{query: query, changed: true})
}

func (q *callbackQueue) confirm() { q.post(queuedCallback{confirm: true}) }
func (q *callbackQueue) cancel()  { q.post(queuedCallback{cancel: true}) }

// stop drops every callback not yet run. One already waiting on the handler
// lock still runs, and the handler's session token turns it away.
func (q *callbackQueue) stop() {
	q.stopOnce.Do(func() { close(q.stopping) })
}

func (q *callbackQueue) post(event queuedCallback) {
	q.mu.Lock()

	if last := len(q.events) - 1; event.changed && last >= 0 && q.events[last].changed {
		q.events[last] = event
	} else {
		q.events = append(q.events, event)
	}

	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *callbackQueue) take() []queuedCallback {
	q.mu.Lock()
	defer q.mu.Unlock()

	events := q.events
	q.events = nil

	return events
}

func (q *callbackQueue) run() {
	for {
		select {
		case <-q.stopping:
			return
		case <-q.wake:
		}

		for _, event := range q.take() {
			select {
			case <-q.stopping:
				return
			default:
			}

			q.call(event)
		}
	}
}

func (q *callbackQueue) call(event queuedCallback) {
	switch {
	case event.changed && q.callbacks.OnQueryChanged != nil:
		q.callbacks.OnQueryChanged(event.query)
	case event.confirm && q.callbacks.OnConfirm != nil:
		q.callbacks.OnConfirm()
	case event.cancel && q.callbacks.OnCancel != nil:
		q.callbacks.OnCancel()
	}
}
