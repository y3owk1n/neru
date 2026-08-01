package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

// callbackTimeout bounds how long a test waits for a subscriber callback.
// Generous enough to survive a loaded CI machine, short enough that a callback
// which never arrives is reported promptly.
const callbackTimeout = 30 * time.Second

// waitForCallbacks blocks until waitGroup reaches zero, failing the test if
// that takes longer than callbackTimeout.
//
// A bare waitGroup.Wait() here would turn the very regression these tests exist
// to catch — a callback that stops firing — into a silent hang that runs out
// the whole package's test timeout and takes every other test in the binary
// down with an unreadable goroutine dump. A bounded wait reports it as what it
// is: this callback did not fire.
func waitForCallbacks(t *testing.T, waitGroup *sync.WaitGroup, what string) {
	t.Helper()

	done := make(chan struct{})

	go func() {
		waitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(callbackTimeout):
		t.Fatalf("timed out after %v waiting for %s", callbackTimeout, what)
	}
}

func TestModifierState_Toggle(t *testing.T) {
	modifierState := state.NewModifierState()

	if got := modifierState.Current(); got != 0 {
		t.Errorf("Expected initial state to be 0, got %v", got)
	}

	modifierState.Toggle(action.ModShift)

	if got := modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift, got %v", got)
	}

	modifierState.Toggle(action.ModShift)

	if got := modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after toggle off, got %v", got)
	}
}

func TestModifierState_ToggleMultiple(t *testing.T) {
	modifierState := state.NewModifierState()

	modifierState.Toggle(action.ModShift)
	modifierState.Toggle(action.ModCmd)

	if got := modifierState.Current(); got != action.ModShift|action.ModCmd {
		t.Errorf("Expected ModShift|ModCmd, got %v", got)
	}
}

func TestModifierState_Reset(t *testing.T) {
	modifierState := state.NewModifierState()

	modifierState.Toggle(action.ModShift)
	modifierState.Toggle(action.ModCmd)

	modifierState.Reset()

	if got := modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after reset, got %v", got)
	}
}

func TestModifierState_ResetNoChange(t *testing.T) {
	modifierState := state.NewModifierState()

	modifierState.Reset()

	if got := modifierState.Current(); got != 0 {
		t.Errorf("Expected 0, got %v", got)
	}
}

func TestModifierState_OnChange(t *testing.T) {
	modifierState := state.NewModifierState()

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	var receivedMods action.Modifiers

	callback := func(mods action.Modifiers) {
		receivedMods = mods

		waitGroup.Done()
	}

	subscriptionID := modifierState.OnChange(callback)
	if subscriptionID == 0 {
		t.Error("Expected non-zero callback ID")
	}

	waitForCallbacks(t, &waitGroup, "the initial OnChange callback")

	if receivedMods != 0 {
		t.Errorf("Expected initial callback with 0, got %v", receivedMods)
	}

	waitGroup.Add(1)
	modifierState.Toggle(action.ModShift)
	waitForCallbacks(t, &waitGroup, "the callback for Toggle(ModShift)")

	if receivedMods != action.ModShift {
		t.Errorf("Expected ModShift, got %v", receivedMods)
	}
}

func TestModifierState_OffChange(t *testing.T) {
	modifierState := state.NewModifierState()

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	var receivedMods action.Modifiers

	callback := func(mods action.Modifiers) {
		receivedMods = mods

		waitGroup.Done()
	}

	subscriptionID := modifierState.OnChange(callback)

	// Wait for the initial async callback (fired by OnChange via goroutine)
	// to complete before unsubscribing, so we don't race on receivedMods.
	waitForCallbacks(t, &waitGroup, "the initial OnChange callback")

	modifierState.OffChange(subscriptionID)

	modifierState.Toggle(action.ModShift)

	if receivedMods != 0 {
		t.Errorf("Expected no callback after unsubscribe, got %v", receivedMods)
	}
}

func TestModifierState_Concurrent(t *testing.T) {
	modifierState := state.NewModifierState()

	var waitGroup sync.WaitGroup
	for range 100 {
		waitGroup.Go(func() {
			modifierState.Toggle(action.ModShift)
		})
	}

	waitForCallbacks(t, &waitGroup, "the concurrent Toggle goroutines")

	// The race detector is the primary check. Also assert the state is still
	// coherent and mutable, so the test is not vacuous without -race.
	settled := modifierState.Current()
	if modifierState.Current() != settled {
		t.Fatal("Current() disagreed with itself with no writer running")
	}

	modifierState.Reset()

	if modifierState.Current() != 0 {
		t.Errorf("Reset() after concurrent access left %v, want 0", modifierState.Current())
	}
}
