package state_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestAppState_ModeExitReason(t *testing.T) {
	_state := state.NewAppState()

	// Initially should be None.
	if got := _state.ConsumeModeExitReason(); got != state.ModeExitReasonNone {
		t.Fatalf("initial ConsumeModeExitReason() = %v, want ModeExitReasonNone", got)
	}

	// Set and consume Completed.
	_state.SetModeExitReason(state.ModeExitReasonCompleted)

	if got := _state.ConsumeModeExitReason(); got != state.ModeExitReasonCompleted {
		t.Fatalf("after Set(Completed) ConsumeModeExitReason() = %v, want Completed", got)
	}

	// Should be reset to None after consumption.

	if got := _state.ConsumeModeExitReason(); got != state.ModeExitReasonNone {
		t.Fatalf("after consume ConsumeModeExitReason() = %v, want ModeExitReasonNone", got)
	}

	// Set and consume Canceled.
	_state.SetModeExitReason(state.ModeExitReasonCancelled)

	if got := _state.ConsumeModeExitReason(); got != state.ModeExitReasonCancelled {
		t.Fatalf(
			"after Set(Canceled) ConsumeModeExitReason() = %v, want Canceled",
			got,
		)
	}

	// Double consume should return None (already reset).
	_state.SetModeExitReason(state.ModeExitReasonCompleted)
	_state.ConsumeModeExitReason() // consume once

	if got := _state.ConsumeModeExitReason(); got != state.ModeExitReasonNone {
		t.Fatalf("double consume = %v, want ModeExitReasonNone", got)
	}
}

func TestAppState_TrySetScreenChangeProcessing_SetsRetryFlag(t *testing.T) {
	_state := state.NewAppState()

	// First call should succeed.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected first TrySet to succeed")
	}

	// Second call should fail and set the pending-retry flag.
	if _state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected second TrySet to fail while processing")
	}

	// Finish should report that a retry is pending and keep processing flag set.
	if !_state.FinishScreenChangeProcessing() {
		t.Error("Expected FinishScreenChangeProcessing to return true (retry pending)")
	}

	// The processing flag is still held, so TrySet fails.
	if _state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to fail because processing flag is still held after retry")
	}

	// The failed TrySet above set pendingRetry again as a side effect.
	// Drain it so we can test a truly clean finish.
	if !_state.FinishScreenChangeProcessing() {
		t.Error(
			"Expected FinishScreenChangeProcessing to return true (side-effect retry from TrySet)",
		)
	}

	// Finishing with no pending retry clears the processing flag.
	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected FinishScreenChangeProcessing to return false (no retry)")
	}

	// The processing flag is now cleared, so TrySet succeeds.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed after clean finish")
	}

	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected FinishScreenChangeProcessing to return false (no retry)")
	}
}

func TestAppState_TrySetScreenChangeProcessing_MultipleRetries(t *testing.T) {
	_state := state.NewAppState()

	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected first TrySet to succeed")
	}

	// Multiple concurrent events should still result in a single retry.
	for range 5 {
		if _state.TrySetScreenChangeProcessing() {
			t.Fatal("Expected TrySet to fail while processing")
		}
	}

	// Only one retry is reported, and the processing flag stays held.
	if !_state.FinishScreenChangeProcessing() {
		t.Error("Expected retry after multiple concurrent events")
	}

	// The processing flag is still held, so TrySet fails.
	if _state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to fail because processing flag is still held")
	}

	// The failed TrySet above set pendingRetry again as a side effect.
	// Drain it so we can test a truly clean finish.
	if !_state.FinishScreenChangeProcessing() {
		t.Error(
			"Expected FinishScreenChangeProcessing to return true (side-effect retry from TrySet)",
		)
	}

	// A clean finish has no retry pending and clears the processing flag.
	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected no retry on clean finish")
	}

	// Now TrySet should succeed.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed after clean finish")
	}

	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected no retry on clean finish")
	}
}

func TestAppState_ResetScreenChangeProcessing_ClearsBothFlags(t *testing.T) {
	_state := state.NewAppState()

	// Acquire processing and set pending retry.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed")
	}

	if _state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to fail while processing")
	}

	// Reset should clear both flags unconditionally.
	_state.ResetScreenChangeProcessing()

	// After reset, TrySet should succeed again.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed after Reset")
	}

	// And Finish should report no pending retry (the old pendingRetry was cleared by Reset).
	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected no retry after Reset cleared pending flag")
	}
}

// TestAppState_ScreenChangeProcessing_MutualExclusion verifies that only one
// goroutine can be inside the critical section at a time by using an atomic
// counter to detect concurrent entry.
func TestAppState_ScreenChangeProcessing_MutualExclusion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	_state := state.NewAppState()

	const (
		numGoroutines = 100
		numCycles     = 100
	)

	var (
		inside     atomic.Int32
		violations atomic.Int32
		waitGroup  sync.WaitGroup
	)

	for range numGoroutines {
		waitGroup.Go(func() {
			for range numCycles {
				if !_state.TrySetScreenChangeProcessing() {
					continue
				}
				// We are now the exclusive owner. If another goroutine is
				// also inside, the counter will exceed 1.
				if inside.Add(1) > 1 {
					violations.Add(1)
				}
				// Simulate work.
				runtime.Gosched()
				inside.Add(-1)
				// Drain retries.
				for _state.FinishScreenChangeProcessing() {
					if inside.Add(1) > 1 {
						violations.Add(1)
					}

					runtime.Gosched()
					inside.Add(-1)
				}
			}
		})
	}

	waitGroup.Wait()

	if v := violations.Load(); v > 0 {
		t.Errorf("Mutual exclusion violated %d time(s)", v)
	}

	// Every owner drained its retries, so the flag must be free for the next
	// screen change.
	if !_state.TrySetScreenChangeProcessing() {
		t.Error("processing flag still held after all goroutines finished")
	}
}

func TestAppState_HotkeyRefreshPending(t *testing.T) {
	_state := state.NewAppState()

	if _state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh pending to be false initially")
	}

	_state.SetHotkeyRefreshPending(true)

	if !_state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to be pending")
	}

	_state.SetHotkeyRefreshPending(false)

	if _state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to not be pending")
	}
}

// TestAppState_Concurrency tests thread-safe access to state.
func TestAppState_Concurrency(t *testing.T) {
	_state := state.NewAppState()

	var waitGroup sync.WaitGroup

	// Concurrent reads and writes
	for range 100 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			_state.SetEnabled(true)
			_ = _state.IsEnabled()
		}()

		go func() {
			defer waitGroup.Done()

			_state.SetMode(domain.ModeHints)
			_ = _state.CurrentMode()
		}()
	}

	waitGroup.Wait()

	// The race detector is the primary check. Also assert the state stayed
	// coherent and writable, so the test can actually fail without -race.
	settled := _state.CurrentMode()
	if _state.CurrentMode() != settled {
		t.Fatal("CurrentMode() disagreed with itself with no writer running")
	}

	_state.SetEnabled(true)

	if !_state.IsEnabled() {
		t.Error("SetEnabled(true) after concurrent access did not take effect")
	}
}

// TestAppState_StateInvariants validates state invariants.
func TestAppState_StateInvariants(t *testing.T) {
	_state := state.NewAppState()

	// Invariant 1: New state should be enabled
	if !_state.IsEnabled() {
		t.Error("Invariant violated: new state should be enabled")
	}

	// Invariant 2: New state should be in ModeIdle
	if _state.CurrentMode() != domain.ModeIdle {
		t.Error("Invariant violated: new state should be in ModeIdle")
	}

	// Invariant 3: Disable() should set enabled to false
	_state.Disable()

	if _state.IsEnabled() {
		t.Error("Invariant violated: Disable() should set enabled to false")
	}

	// Invariant 4: Enable() should set enabled to true
	_state.Enable()

	if !_state.IsEnabled() {
		t.Error("Invariant violated: Enable() should set enabled to true")
	}

	// Invariant 5: SetMode() should update CurrentMode()
	_state.SetMode(domain.ModeHints)

	if _state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: SetMode() should update CurrentMode()")
	}

	// Invariant 6: Mode should persist across enable/disable
	_state.Disable()

	if _state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: mode should persist across disable")
	}

	_state.Enable()

	if _state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: mode should persist across enable")
	}
}

func TestAppState_OnEnabledStateChanged_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	_state := state.NewAppState()

	var waitGroup sync.WaitGroup

	ids := make(chan uint64, 100)

	// Concurrent registrations
	for range 100 {
		waitGroup.Go(func() {
			id := _state.OnEnabledStateChanged(func(enabled bool) {})
			ids <- id
		})
	}

	waitGroup.Wait()
	close(ids)

	idMap := make(map[uint64]bool)

	for id := range ids {
		if idMap[id] {
			t.Errorf("Duplicate callback ID: %d", id)
		}

		idMap[id] = true
	}

	// Concurrent unregistrations
	waitGroup = sync.WaitGroup{}

	for subscriptionID := range idMap {
		waitGroup.Add(1)

		go func(subID uint64) {
			defer waitGroup.Done()

			_state.OffEnabledStateChanged(subID)
		}(subscriptionID)
	}

	waitGroup.Wait()

	// State changes should not panic after all unsubscriptions
	_state.SetEnabled(false)
	_state.SetEnabled(true)
}

func TestAppState_SetEnabledAnnounced_ReaderNeverSeesTheAnnouncementBeforeTheState(t *testing.T) {
	_state := state.NewAppState()
	_state.SetEnabled(false)

	seen := make(chan bool, 1)

	_state.SetEnabledAnnounced(true, func() {
		// A reader that learns of the announcement now, as a watch snapshot
		// does from the event bus, reads the state next.
		go func() { seen <- _state.IsEnabled() }()

		// Give it the time to read, which it would before the change if
		// the announcement went out unlocked.
		time.Sleep(20 * time.Millisecond)
	})

	if !<-seen {
		t.Error("a reader saw the announcement while the state was still disabled")
	}
}
