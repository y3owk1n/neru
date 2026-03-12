//go:build darwin

//nolint:testpackage // This test validates internal queue/dispatcher behavior.
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

	eventTap := &EventTap{
		logger:        zap.NewNop(),
		callback:      func(string) { callbackStarted.Add(1); <-blockCh },
		callbackQueue: make(chan string, 1),
		stopDispatch:  make(chan struct{}),
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

	// Queue one buffered event, then ensure additional enqueue returns quickly
	// instead of blocking the caller.
	eventTap.enqueueKey("second")

	start := time.Now()

	eventTap.enqueueKey("third")

	if time.Since(start) > 25*time.Millisecond {
		t.Fatal("enqueueKey blocked while queue was full")
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
		callbackQueue: make(chan string, 8),
		stopDispatch:  make(chan struct{}),
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
