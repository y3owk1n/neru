package eventtap

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

	et := &EventTap{
		logger:        zap.NewNop(),
		callback:      func(string) { callbackStarted.Add(1); <-blockCh },
		callbackQueue: make(chan string, 1),
		stopDispatch:  make(chan struct{}),
	}
	et.startDispatcher()
	defer et.stopDispatcher()

	et.enqueueKey("first")

	deadline := time.Now().Add(200 * time.Millisecond)
	for callbackStarted.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(1 * time.Millisecond)
	}
	if callbackStarted.Load() == 0 {
		t.Fatal("callback did not start in time")
	}

	// Queue one buffered event, then ensure additional enqueue returns quickly
	// instead of blocking the caller.
	et.enqueueKey("second")

	start := time.Now()
	et.enqueueKey("third")
	if time.Since(start) > 25*time.Millisecond {
		t.Fatal("enqueueKey blocked while queue was full")
	}

	close(blockCh)
}

func TestEventTapEnqueueKey_PreservesOrder(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
		done     = make(chan struct{})
	)

	et := &EventTap{
		logger: zap.NewNop(),
		callback: func(key string) {
			mu.Lock()
			received = append(received, key)
			if len(received) == 4 {
				select {
				case <-done:
				default:
					close(done)
				}
			}
			mu.Unlock()
		},
		callbackQueue: make(chan string, 8),
		stopDispatch:  make(chan struct{}),
	}
	et.startDispatcher()
	defer et.stopDispatcher()

	expected := []string{"u", "i", "j", "k"}
	for _, key := range expected {
		et.enqueueKey(key)
	}

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("did not receive all callbacks in time")
	}

	mu.Lock()
	defer mu.Unlock()
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
