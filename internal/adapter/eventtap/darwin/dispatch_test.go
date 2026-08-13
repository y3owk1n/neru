//go:build darwin

package darwin

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEventTapEnqueueKey_NonBlockingWhenQueueBackedUp(t *testing.T) {
	blockCh := make(chan struct{})

	var callbackStarted atomic.Int32

	eventTap := &EventTap{
		logger:       zap.NewNop(),
		callback:     func(string) { callbackStarted.Add(1); <-blockCh },
		queue:        newUnboundedQueue(),
		stopDispatch: make(chan struct{}),
	}
	eventTap.startDispatcher()

	defer eventTap.stopDispatcher()

	eventTap.enqueueKey("first")

	deadline := time.Now().Add(200 * time.Millisecond)

	for callbackStarted.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(1 * time.Millisecond)
	}

	if callbackStarted.Load() == 0 {
		t.Fatal("callback did not start in time")
	}

	// Verify that enqueueing into the unbounded queue never blocks the caller,
	// even when the consumer is blocked.
	eventTap.enqueueKey("second")

	start := time.Now()

	eventTap.enqueueKey("third")

	if time.Since(start) > 25*time.Millisecond {
		t.Fatal("enqueueKey blocked")
	}

	close(blockCh)
}

func TestEventTapEnqueueKey_PreservesOrder(t *testing.T) {
	var (
		receivedMu sync.Mutex
		received   []string
		done       = make(chan struct{})
	)

	eventTap := &EventTap{
		logger: zap.NewNop(),
		callback: func(key string) {
			receivedMu.Lock()

			received = append(received, key)

			if len(received) == 4 {
				select {
				case <-done:
				default:
					close(done)
				}
			}

			receivedMu.Unlock()
		},
		queue:        newUnboundedQueue(),
		stopDispatch: make(chan struct{}),
	}
	eventTap.startDispatcher()

	defer eventTap.stopDispatcher()

	expected := []string{"u", "i", "j", "k"}
	for _, key := range expected {
		eventTap.enqueueKey(key)
	}

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("did not receive all callbacks in time")
	}

	receivedMu.Lock()

	defer receivedMu.Unlock()

	if len(received) != len(expected) {
		t.Fatalf("received %d keys, want %d", len(received), len(expected))
	}

	for index := range expected {
		if received[index] != expected[index] {
			t.Fatalf(
				"callback order mismatch at index %d: got %q, want %q",
				index,
				received[index],
				expected[index],
			)
		}
	}
}

func TestEventTapEnqueuePassthrough_PreservesSnapshotOrder(t *testing.T) {
	var (
		receivedMu sync.Mutex
		received   []string
		done       = make(chan struct{})
	)

	eventTap := &EventTap{
		logger:       zap.NewNop(),
		queue:        newUnboundedQueue(),
		stopDispatch: make(chan struct{}),
	}
	eventTap.startDispatcher()

	defer eventTap.stopDispatcher()

	appendEvent := func(value string) {
		receivedMu.Lock()
		defer receivedMu.Unlock()

		received = append(received, value)
		if len(received) == 4 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}

	eventTap.callback = func(key string) {
		appendEvent("key:" + key)
	}

	eventTap.enqueueKey("u")
	eventTap.enqueuePassthrough(func() {
		appendEvent("passthrough:first")
	})
	eventTap.enqueueKey("i")
	eventTap.enqueuePassthrough(func() {
		appendEvent("passthrough:second")
	})

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("did not receive all callbacks in time")
	}

	receivedMu.Lock()
	defer receivedMu.Unlock()

	expected := []string{
		"key:u",
		"passthrough:first",
		"key:i",
		"passthrough:second",
	}

	if len(received) != len(expected) {
		t.Fatalf("received %d callbacks, want %d", len(received), len(expected))
	}

	for index := range expected {
		if received[index] != expected[index] {
			t.Fatalf(
				"callback order mismatch at index %d: got %q, want %q",
				index,
				received[index],
				expected[index],
			)
		}
	}
}

// TestEventTap_Destroy_WaitsForAnInFlightKeyDispatch pins the darwin half of
// what makes eventtap.Adapter.Destroy's lock discipline load-bearing.
//
// This backend reaches the wait by a different route than the Linux tap —
// stopDispatcher closes the queue and joins the dispatch goroutine, rather
// than closing the dispatch channel — but the consequence is the same: Destroy
// runs for as long as the key that goroutine is delivering takes, and that key
// is delivered into the mode handler under its own lock. So the adapter must
// not hold its lock across this call.
//
// The Linux twin of this test also re-enters the tap from the in-flight
// callback; this one does not, and the asymmetry is the backends', not the
// test's. The Linux tap guards its whole state with one mutex the dispatch
// goroutine takes, so a Destroy holding it across the wait would park the
// dispatcher. Here nothing the dispatcher runs takes a tap lock at all —
// handleKeyCallback reads the callback under callbackMu and releases it before
// invoking, and Destroy takes callbackMu only after the join — so there is no
// tap-level hold to pin, and the adapter's is the whole of the hazard.
func TestEventTap_Destroy_WaitsForAnInFlightKeyDispatch(t *testing.T) {
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})

	eventTap := &EventTap{
		logger: zap.NewNop(),
		callback: func(string) {
			close(callbackEntered)
			<-releaseCallback
		},
		queue:        newUnboundedQueue(),
		stopDispatch: make(chan struct{}),
	}
	eventTap.startDispatcher()

	eventTap.enqueueKey("u")
	<-callbackEntered

	destroyReturned := make(chan struct{})

	go func() {
		defer close(destroyReturned)

		eventTap.Destroy()
	}()

	select {
	case <-destroyReturned:
		t.Fatal("Destroy returned while a key was still being delivered")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseCallback)

	select {
	case <-destroyReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Destroy never returned after the key finished being delivered")
	}
}
