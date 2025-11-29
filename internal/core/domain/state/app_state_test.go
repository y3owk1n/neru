//go:build !integration

package state_test

import (
	"sync"
	"testing"

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
