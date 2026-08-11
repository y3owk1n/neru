package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// repeatableAction is a held-repeatable binding, so the only thing keeping a
// repeat from starting in these tests is the guard under test.
const repeatableAction = "action scroll_down"

// newMouseButtonRepeatTestHandler builds a handler in recursive-grid mode with
// a mouse button bound to a held-repeat action, held repeat enabled, and delays
// long enough that a repeat goroutine would block on its initial timer rather
// than dispatch during the test. The binding is the shape the guard exists for:
// nothing stops a caller writing it, and scroll is repeatable.
func newMouseButtonRepeatTestHandler() *Handler {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	return newHandlerWithState(handlerState{
		ctx: context.Background(),
		config: &configpkg.Config{
			RecursiveGrid: configpkg.RecursiveGridConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"MouseLeft": {repeatableAction},
				},
			},
			HeldRepeat: configpkg.HeldRepeatConfig{
				Enabled:      true,
				InitialDelay: 10_000,
				Interval:     10_000,
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: &recordingMode{keys: make(chan string, 1)},
		},
		screenBounds:          image.Rect(0, 0, 100, 100),
		executeActionSequence: func(_ string, _ []string) {},
	})
}

// TestHandleKeyPress_MouseButtonStartsNoHeldRepeat pins the guard. A mouse
// button is observed rather than consumed and carries no release event, so the
// "__keyup_" that tears a repeat down never arrives for one. Without the guard
// a button bound to a repeatable action would repeat until the mode exited,
// which is not what clicking a button means.
func TestHandleKeyPress_MouseButtonStartsNoHeldRepeat(t *testing.T) {
	t.Parallel()

	handler := newMouseButtonRepeatTestHandler()

	handler.HandleKeyPress("MouseLeft")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.heldRepeatingKey != "" {
		t.Errorf(
			"mouse button started a held repeat; heldRepeatingKey = %q, want empty",
			handler.heldRepeatingKey,
		)
	}

	if handler.heldRepeatingCancel != nil {
		t.Error("mouse button left a held-repeat cancel func set")
	}
}

// TestHandleKeyPress_KeyboardKeyStillStartsHeldRepeat is the control: the guard
// has to refuse mouse buttons without disarming held repeat for the keys it was
// built for, so the same handler shape with a keyboard binding still repeats.
func TestHandleKeyPress_KeyboardKeyStillStartsHeldRepeat(t *testing.T) {
	t.Parallel()

	handler := newMouseButtonRepeatTestHandler()

	handler.mu.Lock()
	handler.config.RecursiveGrid.Hotkeys = map[string]configpkg.StringOrStringArray{
		"j": {repeatableAction},
	}
	handler.mu.Unlock()

	handler.HandleKeyPress("j")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.heldRepeatingKey != "j" {
		t.Errorf(
			"keyboard key did not start held repeat; heldRepeatingKey = %q, want %q",
			handler.heldRepeatingKey, "j",
		)
	}
}
