package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// TestHandler_ActivateMode_RefusesEveryModeWhenStopped pins that a stopped
// daemon enters no mode from any caller that reaches the handler directly.
// Scroll is the case that was missed before, because it never went through
// the per-mode validation that hints and grid do. The handler here has no
// scroll component wired, so an activation that gets past the guard panics
// instead of passing.
func TestHandler_ActivateMode_RefusesEveryModeWhenStopped(t *testing.T) {
	appState := state.NewAppState()
	appState.SetEnabled(false)

	handler := newHandlerWithState(handlerState{appState: appState})
	handler.modes = map[domain.Mode]Mode{
		domain.ModeScroll: NewScrollMode(&handler.handlerState),
		domain.ModeCustom: NewCustomMode(&handler.handlerState),
	}

	for _, mode := range []domain.Mode{domain.ModeScroll, domain.ModeCustom} {
		handler.ActivateMode(modecmd.Activation{Mode: mode, Name: "any"})

		if got := appState.CurrentMode(); got != domain.ModeIdle {
			t.Fatalf("after activating %s while stopped, mode = %s, want idle",
				domain.ModeString(mode), domain.ModeString(got))
		}
	}
}
