package modes

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestScrollMode_ModeType(t *testing.T) {
	handler := &handlerState{}
	mode := NewScrollMode(handler)

	if mode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", mode.ModeType())
	}
}

func TestScrollMode_InterfaceCompliance(t *testing.T) {
	handler := &handlerState{}
	mode := NewScrollMode(handler)

	if mode == nil {
		t.Fatal("Expected NewScrollMode to return a non-nil mode")
	}

	// Keep a runtime assertion in addition to the compile-time check in
	// scroll_mode.go.
	var interfaceMode Mode = mode
	if interfaceMode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", interfaceMode.ModeType())
	}
}

// TestScrollMode_HandleKey_DoesNothing covers the one dispatch path in the
// package with no journey behind it. Scroll is driven entirely by hotkeys,
// which the dispatcher answers before any mode is reached, so an unbound key
// arriving at the mode must do nothing at all.
//
// A dropped no-op is invisible by construction — no test can tell "does
// nothing" from "is not there". What this catches is the key being routed
// somewhere it should not go: the handler here has no components wired, so
// reaching another mode's key handling panics on a nil field.
func TestScrollMode_HandleKey_DoesNothing(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeScroll)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
	})
	handler.modes = map[domain.Mode]Mode{
		domain.ModeScroll: NewScrollMode(&handler.handlerState),
	}

	handler.handleModeSpecificKey("x")

	if got := appState.CurrentMode(); got != domain.ModeScroll {
		t.Fatalf("mode after an unbound key = %v, want %v", got, domain.ModeScroll)
	}
}
