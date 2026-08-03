//nolint:testpackage // Tests private cleanup ordering.
package modes

import (
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestPerformCommonCleanup_ReleasesStickyModifiersBeforeDisablingEventTap(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	var callOrder []string

	eventTap := &portmocks.MockEventTapPort{
		OnCall: func(label string) {
			if strings.HasSuffix(label, "_down") {
				t.Errorf("unexpected modifier down (%s) during cleanup", label)

				return
			}

			callOrder = append(callOrder, label)
		},
	}

	handler := &Handler{
		logger:         zap.NewNop(),
		config:         &configpkg.Config{},
		appState:       appState,
		modifierState:  state.NewModifierState(),
		overlayManager: &overlay.NoOpManager{},
		eventTap:       eventTap,
	}

	handler.modifierState.Toggle(action.ModCtrl)
	handler.performCommonCleanup()

	if len(callOrder) < 2 {
		t.Fatalf("expected modifier release and disable callbacks, got %v", callOrder)
	}

	if callOrder[0] != keyPartCtrl || callOrder[1] != "disable" {
		t.Fatalf("cleanup order = %v, want [ctrl disable ...]", callOrder)
	}

	if got := handler.modifierState.Current(); got != 0 {
		t.Fatalf("modifierState.Current() = %v, want 0", got)
	}
}
