//nolint:testpackage // Tests repeatPendingDirectAction, which is private.
package modes

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestRepeatPendingDirectAction_ReactivatesMatchingClickAction(t *testing.T) {
	handler := &Handler{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	}

	pendingAction := string(action.NameLeftClick)
	callbackCount := 0

	repeated := handler.repeatPendingDirectAction(
		string(action.NameLeftClick),
		&pendingAction,
		true,
		func() {
			callbackCount++
		},
	)

	if !repeated {
		t.Fatal("expected direct action to trigger repeat re-activation")
	}

	if callbackCount != 1 {
		t.Fatalf("expected re-activation callback once, got %d", callbackCount)
	}

	if !handler.cursorState.WasActionPerformed() {
		t.Fatal("expected click action to be marked as performed")
	}
}

func TestRepeatPendingDirectAction_IgnoresNonMatchingAction(t *testing.T) {
	handler := &Handler{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	}

	pendingAction := string(action.NameLeftClick)
	callbackCount := 0

	repeated := handler.repeatPendingDirectAction(
		string(action.NameRightClick),
		&pendingAction,
		true,
		func() {
			callbackCount++
		},
	)

	if repeated {
		t.Fatal("expected non-matching direct action to skip repeat re-activation")
	}

	if callbackCount != 0 {
		t.Fatalf("expected no re-activation callback, got %d", callbackCount)
	}

	if handler.cursorState.WasActionPerformed() {
		t.Fatal("expected cursor state to remain unchanged for non-matching action")
	}
}

func TestRepeatPendingDirectAction_DoesNotMarkMoveActionPerformed(t *testing.T) {
	handler := &Handler{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	}

	pendingAction := string(action.NameMoveMouseRelative)
	callbackCount := 0

	repeated := handler.repeatPendingDirectAction(
		string(action.NameMoveMouseRelative),
		&pendingAction,
		true,
		func() {
			callbackCount++
		},
	)

	if !repeated {
		t.Fatal("expected move action to trigger repeat re-activation")
	}

	if callbackCount != 1 {
		t.Fatalf("expected re-activation callback once, got %d", callbackCount)
	}

	if handler.cursorState.WasActionPerformed() {
		t.Fatal("expected move action to skip click settle tracking")
	}
}
