package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// callbackTimeout bounds how long a test waits for a subscriber callback.
// Generous enough to survive a loaded CI machine, short enough that a callback
// which never arrives is reported promptly.
const callbackTimeout = 30 * time.Second

// waitForCallbacks blocks until waitGroup reaches zero, failing the test if
// that takes longer than callbackTimeout.
//
// A bare waitGroup.Wait() here would turn a goroutine that never finishes into
// a silent hang that runs out the whole package's test timeout and takes every
// other test in the binary down with an unreadable goroutine dump. A bounded
// wait reports it as what it is, a goroutine that did not finish.
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

func TestModifierState_Concurrent(t *testing.T) {
	modifierState := state.NewModifierState()

	var waitGroup sync.WaitGroup
	for range 101 {
		waitGroup.Go(func() {
			modifierState.Toggle(action.ModShift)
		})
	}

	waitForCallbacks(t, &waitGroup, "the concurrent Toggle goroutines")

	// The race detector is the primary check. An odd number of toggles must
	// also leave the bit set, so a lost update fails without -race.
	if got := modifierState.Current(); got != action.ModShift {
		t.Fatalf("Current() after 101 toggles = %v, want ModShift", got)
	}

	modifierState.Reset()

	if modifierState.Current() != 0 {
		t.Errorf("Reset() after concurrent access left %v, want 0", modifierState.Current())
	}
}
