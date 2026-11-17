package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/heldmotion"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
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
		executeActionSequence: func(_ string, _ []string, _ []string) {},
	})
}

// glideAction is a lone move_mouse_relative with a non-zero delta, the one
// binding shape HeldMotion glides on.
const glideAction = "action move_mouse_relative --dx=10 --dy=0"

// newMouseButtonGlideTestHandler is the repeat handler with a glide binding and
// a live motion controller, so the only thing keeping the button out of the
// glide's held set is the guard.
func newMouseButtonGlideTestHandler() *Handler {
	handler := newMouseButtonRepeatTestHandler()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	handler.config.RecursiveGrid.Hotkeys = map[string]configpkg.StringOrStringArray{
		"MouseLeft": {glideAction},
		"j":         {glideAction},
	}
	handler.motion = heldmotion.New(
		context.Background(),
		&portmocks.MockSystemPort{},
		handler.config.HeldRepeat.Ramp,
		zap.NewNop(),
	).Group("test")

	return handler
}

// TestHandleKeyPress_MouseButtonStartsNoGlide is the held-key glide's half of
// the same rule. The glide is released by the same "__keyup_" a repeat is, so a
// button pressed into its held set would steer the cursor until the mode
// exited. The glide arrived after this binding did, which is why it is pinned
// rather than assumed.
func TestHandleKeyPress_MouseButtonStartsNoGlide(t *testing.T) {
	t.Parallel()

	handler := newMouseButtonGlideTestHandler()

	handler.HandleKeyPress("MouseLeft")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.motion.IsHeld(motionKeyOf("MouseLeft")) {
		t.Error("mouse button entered the glide's held set, which no key-up will clear")
	}
}

// TestHandleKeyPress_KeyboardKeyStillGlides is the control for the guard above:
// a keyboard key on the same binding still reaches the glide.
func TestHandleKeyPress_KeyboardKeyStillGlides(t *testing.T) {
	t.Parallel()

	handler := newMouseButtonGlideTestHandler()

	handler.HandleKeyPress("j")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if !handler.motion.IsHeld(motionKeyOf("j")) {
		t.Error("keyboard key did not reach the glide's held set")
	}
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
