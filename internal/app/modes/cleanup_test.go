package modes

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

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

	handler := newHandlerWithState(handlerState{
		logger:        zap.NewNop(),
		config:        &configpkg.Config{},
		appState:      appState,
		modifierState: state.NewModifierState(),
		eventTap:      eventTap,
	})

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

// TestPerformCommonCleanup_ClearsTheFrameAfterTheContextIsCanceled pins the
// one context the leaving half must ignore. Shutdown cancels the app's root
// context and *then* exits the mode, so a teardown that honored h.ctx would
// leave the last mode's overlay on screen as the daemon quit — and would log
// an error on every clean exit while doing it.
func TestPerformCommonCleanup_ClearsTheFrameAfterTheContextIsCanceled(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	cleared := 0
	overlayPort := &portmocks.MockOverlayPort{
		ClearFrameFunc: func(ctx context.Context) error {
			ctxErr := ctx.Err()
			if ctxErr != nil {
				return ctxErr
			}

			cleared++

			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := newHandlerWithState(handlerState{
		ctx:           ctx,
		logger:        zap.NewNop(),
		config:        &configpkg.Config{},
		appState:      appState,
		modifierState: state.NewModifierState(),
		overlayPort:   overlayPort,
	})

	handler.performCommonCleanup()

	if cleared == 0 {
		t.Fatal("the frame was never cleared: teardown honored the canceled context")
	}
}
