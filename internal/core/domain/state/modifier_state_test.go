package state_test

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

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

	waitGroup.Wait()

	if receivedMods != 0 {
		t.Errorf("Expected initial callback with 0, got %v", receivedMods)
	}

	waitGroup.Add(1)
	modifierState.Toggle(action.ModShift)
	waitGroup.Wait()

	if receivedMods != action.ModShift {
		t.Errorf("Expected ModShift, got %v", receivedMods)
	}
}

func TestModifierState_OffChange(t *testing.T) {
	modifierState := state.NewModifierState()

	var receivedMods action.Modifiers

	callback := func(mods action.Modifiers) {
		receivedMods = mods
	}

	subscriptionID := modifierState.OnChange(callback)
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

	waitGroup.Wait()

	_ = modifierState.Current()
}
