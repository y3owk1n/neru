//go:build windows

package windows

// Windows removes a WH_KEYBOARD_LL hook whose procedure overruns
// LowLevelHooksTimeout, and does it silently: from then on keys reach the
// focused application while Neru believes a mode still has them. So the hook
// procedure classifies the key and returns, and one dispatcher goroutine
// delivers to the handler in order, the shape the macOS and Linux taps have.
// Deleting the dispatcher is silent everywhere except on a Windows desktop
// with a slow hint refresh (ADR 0011).

import (
	"sync/atomic"
	"testing"
	"time"
)

// newTestTap builds a tap whose dispatcher is stopped when the test ends.
func newTestTap(t *testing.T) *EventTap {
	t.Helper()

	eventTap := NewEventTap(nil, nil)
	t.Cleanup(eventTap.Destroy)

	return eventTap
}

func TestEventTap_HandleKey_ReturnsWhileTheHandlerBlocks(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)

	handlerEntered := make(chan struct{})
	releaseHandler := make(chan struct{})

	eventTap.SetHandler(func(string) {
		close(handlerEntered)
		<-releaseHandler
	})

	t.Cleanup(func() { close(releaseHandler) })

	returned := make(chan bool, 1)

	go func() { returned <- eventTap.handleKey("a", false) }()

	select {
	case <-handlerEntered:
	case <-time.After(time.Second):
		t.Fatal("the key never reached the handler")
	}

	select {
	case consumed := <-returned:
		if !consumed {
			t.Error("a bare key was handed to the application instead of consumed")
		}
	case <-time.After(time.Second):
		t.Fatal("handleKey waited on the handler: Windows would have removed the hook")
	}
}

func TestEventTap_DispatchKey_DeliversInOrderOnOneGoroutine(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)

	keys := []string{"h", "i", "n", "t", "s"}
	arrived := make(chan string, len(keys))

	var inFlight atomic.Int32

	eventTap.SetHandler(func(key string) {
		if inFlight.Add(1) != 1 {
			t.Error("the handler was entered concurrently")
		}

		time.Sleep(time.Millisecond)
		inFlight.Add(-1)

		arrived <- key
	})

	for _, key := range keys {
		eventTap.handleKey(key, false)
	}

	for index, want := range keys {
		select {
		case got := <-arrived:
			if got != want {
				t.Fatalf("key %d arrived as %q, want %q: order was not kept", index, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("key %d never arrived", index)
		}
	}
}

// A key that exits the mode runs Disable on the dispatcher itself, while the
// hook thread may be handing over the next key. Neither side may wait on the
// other, and the key read during the exit belongs to nobody.
func TestEventTap_Disable_FromTheHandlerDropsTheKeyInFlight(t *testing.T) {
	t.Parallel()

	// A bare letter, so the tap's key normalization leaves it as written.
	const exitKey = "q"

	eventTap := newTestTap(t)
	eventTap.mu.Lock()
	eventTap.enabled = true
	eventTap.mu.Unlock()

	arrived := make(chan string, 8)
	exitDone := make(chan struct{})

	eventTap.SetHandler(func(key string) {
		arrived <- key

		if key != exitKey {
			return
		}

		hookReturned := make(chan struct{})

		go func() {
			defer close(hookReturned)

			eventTap.handleKey("x", false)
		}()

		select {
		case <-hookReturned:
		case <-time.After(time.Second):
			t.Error("the hook thread waited on the handler during a mode exit")
		}

		eventTap.Disable()
		close(exitDone)
	})

	eventTap.handleKey(exitKey, false)

	select {
	case <-exitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Disable from the handler never returned: the dispatcher deadlocked against itself")
	}

	if got := <-arrived; got != exitKey {
		t.Fatalf("first delivery was %q, want %q", got, exitKey)
	}

	select {
	case got := <-arrived:
		t.Fatalf("%q was delivered after the mode exited", got)
	case <-time.After(50 * time.Millisecond):
	}

	if eventTap.IsEnabled() {
		t.Error("the tap reports enabled after Disable")
	}
}

func TestEventTap_Destroy_WaitsForAnInFlightKeyDispatch(t *testing.T) {
	t.Parallel()

	var eventTap *EventTap

	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})

	eventTap = NewEventTap(func(string) {
		close(callbackEntered)
		<-releaseCallback

		// Parks forever if Destroy is holding et.mu across its wait.
		eventTap.SetHotkeys(nil)
	}, nil)

	eventTap.dispatchKey("u")
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
		t.Fatal("Destroy never returned: the dispatcher is parked on a lock Destroy holds")
	}
}
