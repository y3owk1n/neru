package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestNewAppState(t *testing.T) {
	_state := state.NewAppState()

	if _state == nil {
		t.Fatal("NewAppState() returned nil")
	}

	if !_state.IsEnabled() {
		t.Error("Expected new state to be enabled by default")
	}

	if _state.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode to be ModeIdle, got %v", _state.CurrentMode())
	}
}

func TestAppState_EnableDisable(t *testing.T) {
	_state := state.NewAppState()

	// Test Enable
	_state.Disable()

	if _state.IsEnabled() {
		t.Error("Expected state to be disabled")
	}

	_state.Enable()

	if !_state.IsEnabled() {
		t.Error("Expected state to be enabled")
	}

	// Test SetEnabled
	_state.SetEnabled(false)

	if _state.IsEnabled() {
		t.Error("Expected state to be disabled after SetEnabled(false)")
	}

	_state.SetEnabled(true)

	if !_state.IsEnabled() {
		t.Error("Expected state to be enabled after SetEnabled(true)")
	}
}

func TestAppState_Mode(t *testing.T) {
	_state := state.NewAppState()

	modes := []domain.Mode{
		domain.ModeIdle,
		domain.ModeHints,
		domain.ModeGrid,
	}

	for _, mode := range modes {
		_state.SetMode(mode)

		if _state.CurrentMode() != mode {
			t.Errorf("Expected mode %v, got %v", mode, _state.CurrentMode())
		}
	}
}

func TestAppState_HotkeysRegistered(t *testing.T) {
	_state := state.NewAppState()

	if _state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered initially")
	}

	_state.SetHotkeysRegistered(true)

	if !_state.HotkeysRegistered() {
		t.Error("Expected hotkeys to be registered")
	}

	_state.SetHotkeysRegistered(false)

	if _state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered")
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

	// Finish should report that a retry is pending.
	if !_state.FinishScreenChangeProcessing() {
		t.Error("Expected FinishScreenChangeProcessing to return true (retry pending)")
	}

	// After finish, no retry should be pending.
	// Re-acquire to test a clean finish.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed after finish")
	}

	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected FinishScreenChangeProcessing to return false (no retry)")
	}
}

func TestAppState_FinishScreenChangeProcessing_NoRetryByDefault(t *testing.T) {
	_state := state.NewAppState()
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed")
	}

	// No concurrent event arrived, so finish should return false.
	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected no retry when no concurrent event arrived")
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

	// Only one retry should be reported.
	if !_state.FinishScreenChangeProcessing() {
		t.Error("Expected retry after multiple concurrent events")
	}

	// And the flag should be cleared.
	if !_state.TrySetScreenChangeProcessing() {
		t.Fatal("Expected TrySet to succeed after finish")
	}

	if _state.FinishScreenChangeProcessing() {
		t.Error("Expected no retry on clean finish")
	}
}

func TestAppState_GridOverlayNeedsRefresh(t *testing.T) {
	_state := state.NewAppState()

	if _state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay refresh to be false initially")
	}

	_state.SetGridOverlayNeedsRefresh(true)

	if !_state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to need refresh")
	}

	_state.SetGridOverlayNeedsRefresh(false)

	if _state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to not need refresh")
	}
}

func TestAppState_HintOverlayNeedsRefresh(t *testing.T) {
	_state := state.NewAppState()

	if _state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay refresh to be false initially")
	}

	_state.SetHintOverlayNeedsRefresh(true)

	if !_state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to need refresh")
	}

	_state.SetHintOverlayNeedsRefresh(false)

	if _state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to not need refresh")
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
func TestAppState_Concurrency(_ *testing.T) {
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
}

// Stress tests for robustness.

// TestAppState_RapidModeTransitions tests rapid mode switching.
func TestAppState_RapidModeTransitions(t *testing.T) {
	_state := state.NewAppState()
	modes := []domain.Mode{
		domain.ModeIdle,
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeIdle,
	}

	// Perform 1000 rapid mode transitions
	for range 1000 {
		for _, mode := range modes {
			_state.SetMode(mode)

			if _state.CurrentMode() != mode {
				t.Errorf("Expected mode %v, got %v", mode, _state.CurrentMode())
			}
		}
	}
}

// TestAppState_StateTransitionSequences tests valid state transition sequences.
func TestAppState_StateTransitionSequences(t *testing.T) {
	tests := []struct {
		name      string
		sequence  []domain.Mode
		wantFinal domain.Mode
	}{
		{
			name:      "idle -> hints -> idle",
			sequence:  []domain.Mode{domain.ModeIdle, domain.ModeHints, domain.ModeIdle},
			wantFinal: domain.ModeIdle,
		},
		{
			name:      "idle -> grid -> idle",
			sequence:  []domain.Mode{domain.ModeIdle, domain.ModeGrid, domain.ModeIdle},
			wantFinal: domain.ModeIdle,
		},
		{
			name:      "hints -> grid -> hints",
			sequence:  []domain.Mode{domain.ModeHints, domain.ModeGrid, domain.ModeHints},
			wantFinal: domain.ModeHints,
		},
		{
			name: "complex sequence",
			sequence: []domain.Mode{
				domain.ModeIdle,
				domain.ModeHints,
				domain.ModeGrid,
				domain.ModeHints,
				domain.ModeIdle,
			},
			wantFinal: domain.ModeIdle,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_state := state.NewAppState()

			for _, mode := range testCase.sequence {
				_state.SetMode(mode)

				if _state.CurrentMode() != mode {
					t.Errorf("After SetMode(%v), CurrentMode() = %v", mode, _state.CurrentMode())
				}
			}

			if _state.CurrentMode() != testCase.wantFinal {
				t.Errorf("Final mode = %v, want %v", _state.CurrentMode(), testCase.wantFinal)
			}
		})
	}
}

// TestAppState_ConcurrentStressTest performs intensive concurrent operations.
func TestAppState_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	_state := state.NewAppState()

	var waitGroup sync.WaitGroup

	errors := make(chan error, 1000)

	modes := []domain.Mode{domain.ModeIdle, domain.ModeHints, domain.ModeGrid}

	// Run 1000 concurrent goroutines
	for index := range 1000 {
		waitGroup.Add(1)

		go func(index int) {
			defer waitGroup.Done()

			// Each goroutine performs multiple operations
			for rangeIndex := range 10 {
				// Toggle enabled state
				_state.SetEnabled(index%2 == 0)

				// We cannot assert state.IsEnabled() here due to concurrency.
				// The goal is to ensure thread safety.

				// Cycle through modes
				mode := modes[rangeIndex%len(modes)]
				_state.SetMode(mode)

				// Read current state
				_ = _state.CurrentMode()
				_ = _state.IsEnabled()
			}
		}(index)
	}

	waitGroup.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
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

// TestAppState_MultipleFlags tests concurrent modification of multiple flags.
func TestAppState_MultipleFlags(_ *testing.T) {
	_state := state.NewAppState()

	var waitGroup sync.WaitGroup

	// Concurrently modify enabled flag
	for range 100 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			_state.SetEnabled(true)
		}()

		go func() {
			defer waitGroup.Done()

			_state.SetEnabled(false)
		}()
	}

	waitGroup.Wait()

	// State should be consistent (either true or false, not corrupted)
	_ = _state.IsEnabled() // Should not panic or return invalid value
}

// Callback tests for OnEnabledStateChanged and OffEnabledStateChanged.

// TestAppState_OnEnabledStateChanged_Registration tests callback registration returns valid ID.
func TestAppState_OnEnabledStateChanged_Registration(t *testing.T) {
	_state := state.NewAppState()

	id := _state.OnEnabledStateChanged(func(enabled bool) {})

	if id != 1 {
		t.Errorf("Expected first callback ID to be 1, got %d", id)
	}

	// Register second callback
	id2 := _state.OnEnabledStateChanged(func(enabled bool) {})

	if id2 != 2 {
		t.Errorf("Expected second callback ID to be 2, got %d", id2)
	}
}

// TestAppState_OnEnabledStateChanged_ImmediateInvocation tests callback is called immediately with current state.
func TestAppState_OnEnabledStateChanged_ImmediateInvocation(t *testing.T) {
	_state := state.NewAppState()

	called := make(chan bool, 1)

	_state.OnEnabledStateChanged(func(enabled bool) {
		called <- enabled
	})

	// Wait for immediate callback
	select {
	case enabled := <-called:
		if !enabled {
			t.Error("Expected immediate callback with enabled=true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for immediate callback")
	}
}

// TestAppState_OnEnabledStateChanged_StateChangeNotification tests callbacks are invoked on state changes.
func TestAppState_OnEnabledStateChanged_StateChangeNotification(t *testing.T) {
	_state := state.NewAppState()

	called := make(chan bool, 1)

	_state.OnEnabledStateChanged(func(enabled bool) {
		select {
		case called <- enabled:
		default:
		}
	})

	// Consume immediate callback
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for initial callback")
	}

	// Change state
	_state.SetEnabled(false)

	// Wait for state change notification
	select {
	case enabled := <-called:
		if enabled {
			t.Error("Expected callback with enabled=false after state change")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for state change callback")
	}
}

// TestAppState_OnEnabledStateChanged_NoNotificationOnSameState tests no callback when state doesn't change.
func TestAppState_OnEnabledStateChanged_NoNotificationOnSameState(t *testing.T) {
	_state := state.NewAppState()

	callCount := 0

	var callbackMutex sync.Mutex

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	_state.OnEnabledStateChanged(func(enabled bool) {
		defer waitGroup.Done()

		callbackMutex.Lock()

		callCount++

		callbackMutex.Unlock()
	})

	// Wait for initial callback
	waitGroup.Wait()

	callbackMutex.Lock()

	if callCount != 1 {
		t.Fatalf("Expected 1 initial callback, got %d", callCount)
	}

	callbackMutex.Unlock()

	// Set same state (true -> true)
	_state.SetEnabled(true)

	callbackMutex.Lock()

	if callCount != 1 {
		t.Errorf("Expected still 1 callback after no-op state change, got %d", callCount)
	}

	callbackMutex.Unlock()
}

// TestAppState_OnEnabledStateChanged_MultipleCallbacks tests multiple callbacks are all invoked.
func TestAppState_OnEnabledStateChanged_MultipleCallbacks(t *testing.T) {
	_state := state.NewAppState()

	called1 := make(chan bool, 1)
	called2 := make(chan bool, 1)

	_state.OnEnabledStateChanged(func(enabled bool) {
		select {
		case called1 <- enabled:
		default:
		}
	})

	_state.OnEnabledStateChanged(func(enabled bool) {
		select {
		case called2 <- enabled:
		default:
		}
	})

	// Both should be called immediately
	select {
	case <-called1:
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for callback 1")
	}

	select {
	case <-called2:
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for callback 2")
	}
}

// TestAppState_OnEnabledStateChanged_NilCallback tests nil callback doesn't panic.
func TestAppState_OnEnabledStateChanged_NilCallback(t *testing.T) {
	_state := state.NewAppState()

	// Should not panic and return 0
	subscriptionID := _state.OnEnabledStateChanged(nil)

	if subscriptionID != 0 {
		t.Errorf("Expected nil callback to return 0, got %d", subscriptionID)
	}

	// State changes should not panic
	_state.SetEnabled(false)
	_state.SetEnabled(true)
}

// TestAppState_OffEnabledStateChanged_Unsubscribe tests unsubscribing removes callback.
func TestAppState_OffEnabledStateChanged_Unsubscribe(t *testing.T) {
	_state := state.NewAppState()

	callbackCalled := make(chan bool, 2)

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	subscriptionID := _state.OnEnabledStateChanged(func(enabled bool) {
		defer waitGroup.Done()

		select {
		case callbackCalled <- enabled:
		default:
		}
	})

	// Wait for initial callback
	waitGroup.Wait()

	// Consume the initial callback value
	select {
	case <-callbackCalled:
	default:
	}

	// Unsubscribe
	_state.OffEnabledStateChanged(subscriptionID)

	// Change state - callback should not be called
	_state.SetEnabled(false)

	// Verify callback was not invoked after unsubscribe
	select {
	case value := <-callbackCalled:
		t.Errorf("Callback was invoked after unsubscribe with value: %v", value)
	case <-time.After(100 * time.Millisecond):
		// Expected: callback was not called
	}
}

// TestAppState_OffEnabledStateChanged_InvalidID tests unsubscribing with invalid ID is no-op.
func TestAppState_OffEnabledStateChanged_InvalidID(t *testing.T) {
	_state := state.NewAppState()

	// Should not panic with non-existent ID
	_state.OffEnabledStateChanged(9999)

	// Should not panic with ID that was never issued
	_state.OffEnabledStateChanged(0)
}

// TestAppState_OnEnabledStateChanged_Concurrent tests concurrent callback operations.
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

	// Verify all IDs are unique
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

// TestAppState_CallbackValueCorrectness tests correct state value is passed to callbacks.
func TestAppState_CallbackValueCorrectness(t *testing.T) {
	_state := state.NewAppState()

	var waitGroup sync.WaitGroup

	receivedValues := make(map[bool]int)

	var valuesMutex sync.Mutex

	// Add to waitgroup before registration to catch goroutine callback
	waitGroup.Add(1)

	_state.OnEnabledStateChanged(func(enabled bool) {
		defer waitGroup.Done()

		valuesMutex.Lock()

		receivedValues[enabled]++

		valuesMutex.Unlock()
	})

	// Wait for initial callback
	waitGroup.Wait()

	// Reset for state change callbacks
	waitGroup.Add(2)

	_state.SetEnabled(false)
	_state.SetEnabled(true)

	// Wait for both state change callbacks
	waitGroup.Wait()

	valuesMutex.Lock()

	defer valuesMutex.Unlock()

	if receivedValues[true] != 2 {
		t.Errorf("Expected 2 callbacks with true, got %d", receivedValues[true])
	}

	if receivedValues[false] != 1 {
		t.Errorf("Expected 1 callback with false, got %d", receivedValues[false])
	}
}
