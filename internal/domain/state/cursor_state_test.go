package state_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/state"
)

// TestCursorState_Reset pins that Reset clears the action-performed flag,
// which gates the --repeat settle delay. A flag that survives Reset would
// delay the next session's restore although it performed no action.
func TestCursorState_Reset(t *testing.T) {
	cursorState := state.NewCursorState()

	cursorState.MarkActionPerformed()

	if !cursorState.WasActionPerformed() {
		t.Fatal("WasActionPerformed() = false after MarkActionPerformed()")
	}

	cursorState.Reset()

	if cursorState.WasActionPerformed() {
		t.Error("WasActionPerformed() = true after Reset()")
	}
}
