package modes

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
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
	handler.performCommonCleanup(false)

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

	handler.performCommonCleanup(false)

	if cleared == 0 {
		t.Fatal("the frame was never cleared: teardown honored the canceled context")
	}
}

// TestExitModeForTransition_KeepsTheKeyboard pins what separates the two exits:
// one gives the keyboard back, the other hands it to the mode coming up. See
// the doc comment on exitModeForTransition for why the second one exists.
func TestExitModeForTransition_KeepsTheKeyboard(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		exit        func(*Handler)
		wantEnabled bool
	}{
		{"exitMode", func(h *Handler) { h.exitMode() }, false},
		{"exitModeForTransition", func(h *Handler) { _ = h.exitModeForTransition() }, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			// Monitor select is the mode whose teardown touches the least: the
			// exit under test is the same one for every mode, and a mode with
			// domain state of its own would only need fixtures for it.
			appState.SetMode(domain.ModeMonitorSelect)

			eventTap := &portmocks.MockEventTapPort{}
			_ = eventTap.Enable(context.Background())

			handler := newHandlerWithState(handlerState{
				ctx:           context.Background(),
				logger:        zap.NewNop(),
				config:        &configpkg.Config{},
				appState:      appState,
				modifierState: state.NewModifierState(),
				cursorState:   state.NewCursorState(),
				scroll:        &components.ScrollComponent{Context: &scroll.Context{}},
				eventTap:      eventTap,
			})

			testCase.exit(handler)

			if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
				t.Fatalf("CurrentMode() = %v after the exit, want idle", got)
			}

			if got := eventTap.IsEnabled(); got != testCase.wantEnabled {
				t.Errorf(
					"event tap enabled = %v after %s, want %v",
					got,
					testCase.name,
					testCase.wantEnabled,
				)
			}
		})
	}
}

// TestReleaseKeyboardIfNoModeEntered_ReleasesOnlyWhenNothingWasEntered covers
// the safety net every transition defers: the keyboard exitModeForTransition
// kept belongs to the mode coming up, and an activation that never got there
// must not leave idle holding it.
func TestReleaseKeyboardIfNoModeEntered_ReleasesOnlyWhenNothingWasEntered(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		landedIn    domain.Mode
		wantEnabled bool
	}{
		{"activation gave up", domain.ModeIdle, false},
		{"activation landed", domain.ModeGrid, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			appState.SetMode(testCase.landedIn)

			eventTap := &portmocks.MockEventTapPort{}
			_ = eventTap.Enable(context.Background())

			handler := newHandlerWithState(handlerState{
				ctx:      context.Background(),
				logger:   zap.NewNop(),
				config:   &configpkg.Config{},
				appState: appState,
				eventTap: eventTap,
			})

			handler.releaseKeyboardIfNoModeEntered()

			if got := eventTap.IsEnabled(); got != testCase.wantEnabled {
				t.Errorf("event tap enabled = %v, want %v", got, testCase.wantEnabled)
			}
		})
	}
}
