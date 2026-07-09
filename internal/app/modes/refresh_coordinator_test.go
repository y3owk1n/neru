package modes

import (
	"sync/atomic"
	"testing"
	"time"
)

func waitForCount(t *testing.T, get func() int64, want int64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if get() == want {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("count did not reach %d within %s (last=%d)", want, timeout, get())
}

func TestRefreshCoordinatorCoalesces(t *testing.T) {
	var calls atomic.Int64

	c := newRefreshCoordinator(refreshCoordinatorConfig{
		debounce:  30 * time.Millisecond,
		onRefresh: func() { calls.Add(1) },
	})
	defer c.Stop()

	for range 5 {
		c.Request()
	}

	waitForCount(t, calls.Load, 1, time.Second)

	// No further refreshes after the burst collapses into one.
	time.Sleep(60 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Fatalf("burst should coalesce into one refresh, got %d", got)
	}
}

func TestRefreshCoordinatorDefersWhileMidSelection(t *testing.T) {
	var (
		calls     atomic.Int64
		deferring atomic.Bool
	)

	deferring.Store(true)

	c := newRefreshCoordinator(refreshCoordinatorConfig{
		debounce:    15 * time.Millisecond,
		idleRetry:   15 * time.Millisecond,
		maxDefer:    10 * time.Second, // large: isolate the defer behavior from anti-starvation
		onRefresh:   func() { calls.Add(1) },
		shouldDefer: func() bool { return deferring.Load() },
	})
	defer c.Stop()

	c.Request()

	// While the user is mid-selection, the refresh is held back.
	time.Sleep(80 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("refresh must be deferred while mid-selection, got %d", got)
	}

	// Once selection ends, the pending refresh applies.
	deferring.Store(false)
	waitForCount(t, calls.Load, 1, time.Second)
}

func TestRefreshCoordinatorAntiStarvation(t *testing.T) {
	var calls atomic.Int64

	c := newRefreshCoordinator(refreshCoordinatorConfig{
		debounce:    15 * time.Millisecond,
		idleRetry:   15 * time.Millisecond,
		maxDefer:    50 * time.Millisecond,
		onRefresh:   func() { calls.Add(1) },
		shouldDefer: func() bool { return true }, // never a neutral point
	})
	defer c.Stop()

	c.Request()

	// Even though shouldDefer is always true, the refresh must eventually fire so
	// stale hints do not linger forever.
	waitForCount(t, calls.Load, 1, time.Second)
}

func TestRefreshCoordinatorStormStillFires(t *testing.T) {
	var calls atomic.Int64

	c := newRefreshCoordinator(refreshCoordinatorConfig{
		debounce:  20 * time.Millisecond,
		maxDefer:  100 * time.Millisecond,
		onRefresh: func() { calls.Add(1) },
	})
	defer c.Stop()

	// Continuous sub-debounce storm: request faster than the debounce for longer
	// than maxDefer. A pure trailing debounce would never fire; the max-defer cap
	// must force at least one refresh.
	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 40 {
			c.Request()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	<-done

	if got := calls.Load(); got < 1 {
		t.Fatalf("sustained storm produced no refresh; got %d", got)
	}
}

func TestRefreshCoordinatorStopIsNoOp(t *testing.T) {
	var calls atomic.Int64

	c := newRefreshCoordinator(refreshCoordinatorConfig{
		debounce:  15 * time.Millisecond,
		onRefresh: func() { calls.Add(1) },
	})

	c.Stop()
	c.Request()

	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("Request after Stop must not fire, got %d", got)
	}
}
