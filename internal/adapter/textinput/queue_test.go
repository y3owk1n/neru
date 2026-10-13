package textinput

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/ports"
)

// recorder collects the callbacks a queue runs, in order.
type recorder struct {
	mu    sync.Mutex
	calls []string
	ran   chan struct{}
}

func newRecorder() *recorder { return &recorder{ran: make(chan struct{}, 16)} }

func (r *recorder) record(call string) {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()

	r.ran <- struct{}{}
}

func (r *recorder) callbacks() ports.TextInputCallbacks {
	return ports.TextInputCallbacks{
		OnQueryChanged: func(query string) { r.record("query " + query) },
		OnConfirm:      func() { r.record("confirm") },
		OnCancel:       func() { r.record("cancel") },
	}
}

func (r *recorder) waitFor(t *testing.T, count int) []string {
	t.Helper()

	for range count {
		select {
		case <-r.ran:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %d callbacks", count)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.calls)
}

// TestCallbackQueue_RunsCallbacksInOrderWithTheLatestQuery pins what the
// handler sees from a field: every Return and Escape, in the order typed, and
// of a run of queries only the latest, which is all the search needs.
func TestCallbackQueue_RunsCallbacksInOrderWithTheLatestQuery(t *testing.T) {
	calls := newRecorder()
	inFirst := make(chan struct{})
	gate := make(chan struct{})

	queue := newCallbackQueue(ports.TextInputCallbacks{
		OnQueryChanged: func(query string) {
			if query == "s" {
				close(inFirst)
				<-gate
			}

			calls.record("query " + query)
		},
		OnConfirm: calls.callbacks().OnConfirm,
	})
	defer queue.stop()

	// The first query holds the queue while the rest arrive behind it.
	queue.queryChanged("s")
	<-inFirst
	queue.queryChanged("sa")
	queue.queryChanged("sav")
	queue.confirm()
	close(gate)

	got := calls.waitFor(t, 3)

	want := []string{"query s", "query sav", "confirm"}
	if !slices.Equal(got, want) {
		t.Errorf("callbacks = %v, want %v", got, want)
	}
}

// TestCallbackQueue_RunsNothingAfterStop pins that a stop is final, even for
// callbacks queued behind one still running.
func TestCallbackQueue_RunsNothingAfterStop(t *testing.T) {
	calls := newRecorder()
	inFirst := make(chan struct{})
	release := make(chan struct{})

	queue := newCallbackQueue(ports.TextInputCallbacks{
		OnQueryChanged: func(query string) {
			close(inFirst)
			<-release
			calls.record("query " + query)
		},
		OnCancel: calls.callbacks().OnCancel,
	})

	queue.queryChanged("s")
	<-inFirst
	queue.cancel()
	queue.stop()
	close(release)

	got := calls.waitFor(t, 1)

	time.Sleep(50 * time.Millisecond)

	calls.mu.Lock()
	defer calls.mu.Unlock()

	if !slices.Equal(calls.calls, []string{"query s"}) || len(got) != 1 {
		t.Errorf("callbacks = %v, want only the one running at stop", calls.calls)
	}
}

// TestCallbackQueue_StopFromInsideACallbackReturns pins the handler's path:
// a callback stops the session, which stops this queue from the goroutine
// running the callback, so stop must not wait for that goroutine.
func TestCallbackQueue_StopFromInsideACallbackReturns(t *testing.T) {
	stopped := make(chan struct{})

	var queue *callbackQueue

	queue = newCallbackQueue(ports.TextInputCallbacks{
		OnCancel: func() {
			queue.stop()
			close(stopped)
		},
	})

	queue.cancel()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stop called from a callback did not return")
	}
}
