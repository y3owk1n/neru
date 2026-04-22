//nolint:testpackage // Tests private cleanup ordering.
package modes

import (
	"testing"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

func TestPerformCommonCleanup_ReleasesStickyModifiersBeforeDisablingEventTap(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	var callOrder []string

	handler := &Handler{
		logger:         zap.NewNop(),
		config:         &configpkg.Config{},
		appState:       appState,
		modifierState:  state.NewModifierState(),
		overlayManager: &overlay.NoOpManager{},
		disableEventTap: func() {
			callOrder = append(callOrder, "disable")
		},
		postModifierEvent: func(modifier string, isDown bool) {
			callOrder = append(callOrder, modifier)
			if isDown {
				t.Fatalf("unexpected modifier down for %s during cleanup", modifier)
			}
		},
	}

	handler.modifierState.Toggle(action.ModCtrl)
	handler.performCommonCleanup()

	if len(callOrder) < 2 {
		t.Fatalf("expected modifier release and disable callbacks, got %v", callOrder)
	}

	if callOrder[0] != "ctrl" || callOrder[1] != "disable" {
		t.Fatalf("cleanup order = %v, want [ctrl disable ...]", callOrder)
	}

	if got := handler.modifierState.Current(); got != 0 {
		t.Fatalf("modifierState.Current() = %v, want 0", got)
	}
}
