package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestNewAppState(t *testing.T) {
	state := state.NewAppState()

	if state == nil {
		t.Fatal("NewAppState() returned nil")
	}

	if !state.IsEnabled() {
		t.Error("Expected new state to be enabled by default")
	}

	if state.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode to be ModeIdle, got %v", state.CurrentMode())
	}
}

func TestAppState_EnableDisable(t *testing.T) {
	state := state.NewAppState()

	// Test Enable
	state.Disable()

	if state.IsEnabled() {
		t.Error("Expected state to be disabled")
	}

	state.Enable()

	if !state.IsEnabled() {
		t.Error("Expected state to be enabled")
	}

	// Test SetEnabled
	state.SetEnabled(false)

	if state.IsEnabled() {
		t.Error("Expected state to be disabled after SetEnabled(false)")
	}

	state.SetEnabled(true)

	if !state.IsEnabled() {
		t.Error("Expected state to be enabled after SetEnabled(true)")
	}
}

func TestAppState_Mode(t *testing.T) {
	state := state.NewAppState()

	modes := []domain.Mode{
		domain.ModeIdle,
		domain.ModeHints,
		domain.ModeGrid,
	}

	for _, mode := range modes {
		state.SetMode(mode)

		if state.CurrentMode() != mode {
			t.Errorf("Expected mode %v, got %v", mode, state.CurrentMode())
		}
	}
}

func TestAppState_HotkeysRegistered(t *testing.T) {
	state := state.NewAppState()

	if state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered initially")
	}

	state.SetHotkeysRegistered(true)

	if !state.HotkeysRegistered() {
		t.Error("Expected hotkeys to be registered")
	}

	state.SetHotkeysRegistered(false)

	if state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered")
	}
}

func TestAppState_ScreenChangeProcessing(t *testing.T) {
	state := state.NewAppState()

	if state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be false initially")
	}

	state.SetScreenChangeProcessing(true)

	if !state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be true")
	}

	state.SetScreenChangeProcessing(false)

	if state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be false")
	}
}

func TestAppState_GridOverlayNeedsRefresh(t *testing.T) {
	state := state.NewAppState()

	if state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay refresh to be false initially")
	}

	state.SetGridOverlayNeedsRefresh(true)

	if !state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to need refresh")
	}

	state.SetGridOverlayNeedsRefresh(false)

	if state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to not need refresh")
	}
}

func TestAppState_HintOverlayNeedsRefresh(t *testing.T) {
	state := state.NewAppState()

	if state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay refresh to be false initially")
	}

	state.SetHintOverlayNeedsRefresh(true)

	if !state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to need refresh")
	}

	state.SetHintOverlayNeedsRefresh(false)

	if state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to not need refresh")
	}
}

func TestAppState_HotkeyRefreshPending(t *testing.T) {
	state := state.NewAppState()

	if state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh pending to be false initially")
	}

	state.SetHotkeyRefreshPending(true)

	if !state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to be pending")
	}

	state.SetHotkeyRefreshPending(false)

	if state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to not be pending")
	}
}

// TestAppState_Concurrency tests thread-safe access to state.
func TestAppState_Concurrency(_ *testing.T) {
	state := state.NewAppState()

	var waitGroup sync.WaitGroup

	// Concurrent reads and writes
	for range 100 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			state.SetEnabled(true)
			_ = state.IsEnabled()
		}()

		go func() {
			defer waitGroup.Done()

			state.SetMode(domain.ModeHints)
			_ = state.CurrentMode()
		}()
	}

	waitGroup.Wait()
}

// Stress tests for robustness.

// TestAppState_RapidModeTransitions tests rapid mode switching.
func TestAppState_RapidModeTransitions(t *testing.T) {
	state := state.NewAppState()
	modes := []domain.Mode{
		domain.ModeIdle,
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeIdle,
	}

	// Perform 1000 rapid mode transitions
	for range 1000 {
		for _, mode := range modes {
			state.SetMode(mode)

			if state.CurrentMode() != mode {
				t.Errorf("Expected mode %v, got %v", mode, state.CurrentMode())
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
			state := state.NewAppState()

			for _, mode := range testCase.sequence {
				state.SetMode(mode)

				if state.CurrentMode() != mode {
					t.Errorf("After SetMode(%v), CurrentMode() = %v", mode, state.CurrentMode())
				}
			}

			if state.CurrentMode() != testCase.wantFinal {
				t.Errorf("Final mode = %v, want %v", state.CurrentMode(), testCase.wantFinal)
			}
		})
	}
}

// TestAppState_ConcurrentStressTest performs intensive concurrent operations.
func TestAppState_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	state := state.NewAppState()

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
				state.SetEnabled(index%2 == 0)

				// We cannot assert state.IsEnabled() here due to concurrency.
				// The goal is to ensure thread safety.

				// Cycle through modes
				mode := modes[rangeIndex%len(modes)]
				state.SetMode(mode)

				// Read current state
				_ = state.CurrentMode()
				_ = state.IsEnabled()
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
	state := state.NewAppState()

	// Invariant 1: New state should be enabled
	if !state.IsEnabled() {
		t.Error("Invariant violated: new state should be enabled")
	}

	// Invariant 2: New state should be in ModeIdle
	if state.CurrentMode() != domain.ModeIdle {
		t.Error("Invariant violated: new state should be in ModeIdle")
	}

	// Invariant 3: Disable() should set enabled to false
	state.Disable()

	if state.IsEnabled() {
		t.Error("Invariant violated: Disable() should set enabled to false")
	}

	// Invariant 4: Enable() should set enabled to true
	state.Enable()

	if !state.IsEnabled() {
		t.Error("Invariant violated: Enable() should set enabled to true")
	}

	// Invariant 5: SetMode() should update CurrentMode()
	state.SetMode(domain.ModeHints)

	if state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: SetMode() should update CurrentMode()")
	}

	// Invariant 6: Mode should persist across enable/disable
	state.Disable()

	if state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: mode should persist across disable")
	}

	state.Enable()

	if state.CurrentMode() != domain.ModeHints {
		t.Error("Invariant violated: mode should persist across enable")
	}
}

// TestAppState_MultipleFlags tests concurrent modification of multiple flags.
func TestAppState_MultipleFlags(_ *testing.T) {
	state := state.NewAppState()

	var waitGroup sync.WaitGroup

	// Concurrently modify enabled flag
	for range 100 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			state.SetEnabled(true)
		}()

		go func() {
			defer waitGroup.Done()

			state.SetEnabled(false)
		}()
	}

	waitGroup.Wait()

	// State should be consistent (either true or false, not corrupted)
	_ = state.IsEnabled() // Should not panic or return invalid value
}

// Callback tests for OnEnabledStateChanged and OffEnabledStateChanged.

// TestAppState_OnEnabledStateChanged_Registration tests callback registration returns valid ID.
func TestAppState_OnEnabledStateChanged_Registration(t *testing.T) {
	state := state.NewAppState()

	id := state.OnEnabledStateChanged(func(enabled bool) {})

	if id != 0 {
		t.Errorf("Expected first callback ID to be 0, got %d", id)
	}

	// Register second callback
	id2 := state.OnEnabledStateChanged(func(enabled bool) {})

	if id2 != 1 {
		t.Errorf("Expected second callback ID to be 1, got %d", id2)
	}
}

// TestAppState_OnEnabledStateChanged_ImmediateInvocation tests callback is called immediately with current state.
func TestAppState_OnEnabledStateChanged_ImmediateInvocation(t *testing.T) {
	state := state.NewAppState()

	called := make(chan bool, 1)

	state.OnEnabledStateChanged(func(enabled bool) {
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
	state := state.NewAppState()

	called := make(chan bool, 1)

	state.OnEnabledStateChanged(func(enabled bool) {
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
	state.SetEnabled(false)

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
	state := state.NewAppState()

	callCount := 0

	var callbackMutex sync.Mutex

	state.OnEnabledStateChanged(func(enabled bool) {
		callbackMutex.Lock()

		callCount++

		callbackMutex.Unlock()
	})

	// Wait for initial callback
	time.Sleep(50 * time.Millisecond)

	callbackMutex.Lock()

	if callCount != 1 {
		t.Fatalf("Expected 1 initial callback, got %d", callCount)
	}

	callbackMutex.Unlock()

	// Set same state (true -> true)
	state.SetEnabled(true)

	time.Sleep(50 * time.Millisecond)

	callbackMutex.Lock()

	if callCount != 1 {
		t.Errorf("Expected still 1 callback after no-op state change, got %d", callCount)
	}

	callbackMutex.Unlock()
}

// TestAppState_OnEnabledStateChanged_MultipleCallbacks tests multiple callbacks are all invoked.
func TestAppState_OnEnabledStateChanged_MultipleCallbacks(t *testing.T) {
	state := state.NewAppState()

	called1 := make(chan bool, 1)
	called2 := make(chan bool, 1)

	state.OnEnabledStateChanged(func(enabled bool) {
		select {
		case called1 <- enabled:
		default:
		}
	})

	state.OnEnabledStateChanged(func(enabled bool) {
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
	state := state.NewAppState()

	// Should not panic
	subscriptionID := state.OnEnabledStateChanged(nil)

	// State changes should not panic
	state.SetEnabled(false)
	state.SetEnabled(true)

	// Just verify we got an ID
	_ = subscriptionID
}

// TestAppState_OffEnabledStateChanged_Unsubscribe tests unsubscribing removes callback.
func TestAppState_OffEnabledStateChanged_Unsubscribe(t *testing.T) {
	state := state.NewAppState()

	callCount := 0

	var callbackMutex sync.Mutex

	subscriptionID := state.OnEnabledStateChanged(func(enabled bool) {
		callbackMutex.Lock()

		callCount++

		callbackMutex.Unlock()
	})

	// Wait for initial callback
	time.Sleep(50 * time.Millisecond)

	// Unsubscribe
	state.OffEnabledStateChanged(subscriptionID)

	// Change state
	state.SetEnabled(false)

	time.Sleep(50 * time.Millisecond)

	callbackMutex.Lock()

	if callCount != 1 {
		t.Errorf("Expected 1 callback (initial only), got %d", callCount)
	}

	callbackMutex.Unlock()
}

// TestAppState_OffEnabledStateChanged_InvalidID tests unsubscribing with invalid ID is no-op.
func TestAppState_OffEnabledStateChanged_InvalidID(t *testing.T) {
	state := state.NewAppState()

	// Should not panic with non-existent ID
	state.OffEnabledStateChanged(9999)

	// Should not panic with ID that was never issued
	state.OffEnabledStateChanged(0)
}

// TestAppState_OnEnabledStateChanged_Concurrent tests concurrent callback operations.
func TestAppState_OnEnabledStateChanged_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	state := state.NewAppState()

	var waitGroup sync.WaitGroup

	ids := make(chan uint64, 100)

	// Concurrent registrations
	for range 100 {
		waitGroup.Go(func() {
			id := state.OnEnabledStateChanged(func(enabled bool) {})
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

			state.OffEnabledStateChanged(subID)
		}(subscriptionID)
	}

	waitGroup.Wait()

	// State changes should not panic after all unsubscriptions
	state.SetEnabled(false)
	state.SetEnabled(true)
}

// TestAppState_CallbackReentrancy tests that SetEnabled can be called from callback without deadlock.
func TestAppState_CallbackReentrancy(t *testing.T) {
	state := state.NewAppState()
	called := make(chan bool, 1)

	state.OnEnabledStateChanged(func(enabled bool) {
		// Try to trigger another state change from callback
		// This should not deadlock
		if enabled {
			state.SetEnabled(false)
		}

		select {
		case called <- true:
		default:
		}
	})

	// Consume initial callback
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for initial callback")
	}

	// Wait for potential reentrant callback
	time.Sleep(100 * time.Millisecond)

	// Verify state was changed
	if state.IsEnabled() {
		t.Error("Expected state to be false after reentrant SetEnabled(false)")
	}
}

// TestAppState_CallbackValueCorrectness tests correct state value is passed to callbacks.
func TestAppState_CallbackValueCorrectness(t *testing.T) {
	state := state.NewAppState()

	receivedValues := make(chan bool, 3)

	state.OnEnabledStateChanged(func(enabled bool) {
		receivedValues <- enabled
	})

	// Should receive: true (initial), false, true
	state.SetEnabled(false)
	state.SetEnabled(true)

	time.Sleep(100 * time.Millisecond)
	close(receivedValues)

	expected := []bool{true, false, true}
	idx := 0

	for value := range receivedValues {
		if idx >= len(expected) {
			t.Errorf("Unexpected callback with value %v", value)

			continue
		}

		if value != expected[idx] {
			t.Errorf("Expected callback %d to receive %v, got %v", idx, expected[idx], value)
		}

		idx++
	}

	if idx != len(expected) {
		t.Errorf("Expected %d callbacks, got %d", len(expected), idx)
	}
}
