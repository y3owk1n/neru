//nolint:testpackage // Tests internal function parseModifierEvent
package modes

import (
	"testing"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestParseModifierEvent(t *testing.T) {
	tests := []struct {
		key      string
		wantMod  action.Modifiers
		wantDown bool
		wantOk   bool
	}{
		// Down events
		{"__modifier_shift_down", action.ModShift, true, true},
		{"__modifier_cmd_down", action.ModCmd, true, true},
		{"__modifier_alt_down", action.ModAlt, true, true},
		{"__modifier_ctrl_down", action.ModCtrl, true, true},
		{"__modifier_CMD_down", action.ModCmd, true, true},
		{"__modifier_Shift_down", action.ModShift, true, true},
		// Up events
		{"__modifier_shift_up", action.ModShift, false, true},
		{"__modifier_cmd_up", action.ModCmd, false, true},
		{"__modifier_alt_up", action.ModAlt, false, true},
		{"__modifier_ctrl_up", action.ModCtrl, false, true},
		{"__modifier_CMD_up", action.ModCmd, false, true},
		// Invalid — when ok=false, mod and isDown are undefined;
		// we test the actual return values for completeness.
		{"__modifier_shift", 0, false, false},
		{"__modifier_cmd", 0, false, false},
		{"__modifier_foo_down", 0, true, false}, // has _down suffix but unknown modifier
		{"__modifier_foo_up", 0, false, false},  // has _up suffix but unknown modifier
		{"__modifier", 0, false, false},
		{"shift", 0, false, false},
		{"cmd", 0, false, false},
		{"", 0, false, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.key, func(t *testing.T) {
			mod, isDown, ok := parseModifierEvent(testCase.key)
			if ok != testCase.wantOk {
				t.Errorf(
					"parseModifierEvent(%q) ok = %v, want %v",
					testCase.key,
					ok,
					testCase.wantOk,
				)

				return
			}

			if mod != testCase.wantMod {
				t.Errorf(
					"parseModifierEvent(%q) mod = %v, want %v",
					testCase.key,
					mod,
					testCase.wantMod,
				)
			}

			if isDown != testCase.wantDown {
				t.Errorf(
					"parseModifierEvent(%q) isDown = %v, want %v",
					testCase.key,
					isDown,
					testCase.wantDown,
				)
			}
		})
	}
}

// newTestHandler creates a minimal Handler suitable for testing sticky modifier behavior.
// The debounceNotify channel is buffered so the timer callback never blocks.
func newTestHandler() *Handler {
	return &Handler{
		logger:         zap.NewNop(),
		modifierState:  state.NewModifierState(),
		debounceNotify: make(chan struct{}, 16),
		config: &configpkg.Config{
			StickyModifiers: configpkg.StickyModifiersConfig{
				Enabled:        true,
				TapMaxDuration: 300,
			},
		},
	}
}

// awaitDebounce blocks until the debounce timer callback signals completion.
// It must be called WITHOUT holding h.mu (the timer callback needs the lock).
// Times out after 1s to prevent tests from hanging.
func awaitDebounce(t *testing.T, h *Handler) {
	t.Helper()

	select {
	case <-h.debounceNotify:
		// Timer callback completed.
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for debounce timer to complete")
	}
}

func TestHandleModifierToggle_SingleModifier(t *testing.T) {
	testHandler := newTestHandler()
	// Shift↓ → Shift↑ should toggle Shift on.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift after single tap, got %v", got)
	}
	// Shift↓ → Shift↑ again should toggle Shift off.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after double tap, got %v", got)
	}
}

func TestHandleModifierToggle_SimultaneousTwoModifiers(t *testing.T) {
	testHandler := newTestHandler()
	// Simulate pressing Cmd and Shift at the same time:
	// Cmd↓ → Shift↓ → Cmd↑ → Shift↑
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)

	got := testHandler.modifierState.Current()

	want := action.ModCmd | action.ModShift
	if got != want {
		t.Errorf(
			"Expected Cmd+Shift (%v) after simultaneous press, got %v",
			want,
			got,
		)
	}
}

func TestHandleModifierToggle_SimultaneousThreeModifiers(t *testing.T) {
	testHandler := newTestHandler()
	// Simulate pressing Cmd+Shift+Alt at the same time:
	// Cmd↓ → Shift↓ → Alt↓ → Cmd↑ → Shift↑ → Alt↑
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_alt_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)

	got := testHandler.modifierState.Current()

	want := action.ModCmd | action.ModShift | action.ModAlt
	if got != want {
		t.Errorf(
			"Expected Cmd+Shift+Alt (%v) after simultaneous press, got %v",
			want,
			got,
		)
	}
}

func TestHandleModifierToggle_SimultaneousFourModifiers(t *testing.T) {
	testHandler := newTestHandler()
	// Simulate pressing all four modifiers simultaneously:
	// Cmd↓ → Shift↓ → Alt↓ → Ctrl↓ → Cmd↑ → Shift↑ → Alt↑ → Ctrl↑
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_ctrl_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_alt_up")
	testHandler.handleModifierToggle("__modifier_ctrl_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)

	got := testHandler.modifierState.Current()

	want := action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl
	if got != want {
		t.Errorf(
			"Expected all four modifiers (%v) after simultaneous press, got %v",
			want,
			got,
		)
	}
}

func TestHandleModifierToggle_SimultaneousReleaseOrderReversed(t *testing.T) {
	testHandler := newTestHandler()
	// Release order reversed from press order:
	// Cmd↓ → Shift↓ → Alt↓ → Alt↑ → Shift↑ → Cmd↑
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_alt_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)
	awaitDebounce(t, testHandler)

	got := testHandler.modifierState.Current()

	want := action.ModCmd | action.ModShift | action.ModAlt
	if got != want {
		t.Errorf(
			"Expected Cmd+Shift+Alt (%v) with reversed release order, got %v",
			want,
			got,
		)
	}
}

func TestHandleModifierToggle_RegularKeyCancelsAllPending(t *testing.T) {
	testHandler := newTestHandler()
	// Cmd↓ → Shift↓ → (regular key cancels) → Cmd↑ → Shift↑
	// Neither should toggle because a regular key intervened.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.cancelPendingModifierToggle() // simulates regular key press
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after regular key canceled pending, got %v", got)
	}
}

func TestHandleModifierToggle_ModifierUsedInChordDoesNotToggleOnRelease(t *testing.T) {
	testHandler := newTestHandler()

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.markHeldModifiersUsedInChord()
	testHandler.cancelPendingModifierToggle()
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after chord usage, got %v", got)
	}
}

func TestHandleModifierToggle_ModifierCanToggleAgainAfterChordUse(t *testing.T) {
	testHandler := newTestHandler()

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.markHeldModifiersUsedInChord()
	testHandler.cancelPendingModifierToggle()
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModCmd {
		t.Errorf("Expected ModCmd after fresh tap following chord use, got %v", got)
	}
}

func TestHandleModifierToggle_SuppressedActivationModifierDoesNotToggleUntilRepressed(
	t *testing.T,
) {
	testHandler := newTestHandler()

	testHandler.SuppressModifiersUntilReleased(action.ModCmd | action.ModShift)

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after suppressed activation modifier release, got %v", got)
	}

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModCmd {
		t.Errorf("Expected ModCmd after fresh press following suppression, got %v", got)
	}
}

func TestHandleModifierToggle_SuppressionExpiresWithoutRelease(t *testing.T) {
	testHandler := newTestHandler()

	testHandler.SuppressModifiersUntilReleased(action.ModCmd)
	testHandler.suppressedUntil = time.Now().Add(-time.Millisecond)

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModCmd {
		t.Errorf("Expected ModCmd after expired suppression, got %v", got)
	}
}

func TestHandleModifierToggle_DisabledConfig(t *testing.T) {
	testHandler := newTestHandler()
	testHandler.config.StickyModifiers.Enabled = false

	handled := testHandler.handleModifierToggle("__modifier_shift_down")
	if handled {
		t.Error("Expected handleModifierToggle to return false when disabled")
	}
}

func TestHandleModifierToggle_HeldTooLong(t *testing.T) {
	testHandler := newTestHandler()
	testHandler.config.StickyModifiers.TapMaxDuration = 100 // 100ms threshold
	// Simulate key down, then manually backdate the timestamp to simulate a long hold.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	// Overwrite the recorded time to 200ms ago (exceeds 100ms threshold).
	testHandler.pendingModifierKeys[action.ModShift] = time.Now().Add(-200 * time.Millisecond)
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after holding too long, got %v", got)
	}
}

func TestHandleModifierToggle_HeldWithinThreshold(t *testing.T) {
	testHandler := newTestHandler()
	testHandler.config.StickyModifiers.TapMaxDuration = 500 // 500ms threshold
	// Quick tap — should toggle.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift after quick tap, got %v", got)
	}
}

func TestHandleModifierToggle_ZeroThresholdAlwaysToggles(t *testing.T) {
	testHandler := newTestHandler()
	testHandler.config.StickyModifiers.TapMaxDuration = 0 // disabled
	// Even a "long hold" should toggle when threshold is 0.
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.pendingModifierKeys[action.ModShift] = time.Now().Add(-10 * time.Second)
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.mu.Unlock()
	awaitDebounce(t, testHandler)

	if got := testHandler.modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift with zero threshold (always toggle), got %v", got)
	}
}

// TestHandleModifierToggle_KarabinerScenario simulates the exact event sequence
// that Karabiner sends when remapping Option+h → Left Arrow:
// alt_down → alt_up → Left (regular key arrives after modifier released).
// The debounce window should allow the regular key to cancel the toggle.
func TestHandleModifierToggle_KarabinerScenario(t *testing.T) {
	testHandler := newTestHandler()

	// Phase 1: Karabiner sends alt_down, then immediately alt_up (flag cleared before arrow)
	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_alt_up")

	// Phase 2: The remapped arrow key arrives during the debounce window.
	// This calls cancelPendingModifierToggle, which should stop the timer.
	testHandler.cancelPendingModifierToggle()
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 (Karabiner remapped key should cancel toggle), got %v", got)
	}
}

// TestHandleModifierToggle_DebounceTimerCancelledByRegularKey verifies that a
// regular key press arriving during the debounce window prevents the toggle,
// even though the modifier down/up sequence was clean.
func TestHandleModifierToggle_DebounceTimerCancelledByRegularKey(t *testing.T) {
	testHandler := newTestHandler()

	testHandler.mu.Lock()
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")

	// Simulate a regular key press arriving during the debounce window.
	// This should cancel both the pending key map entry and the timer.
	testHandler.cancelPendingModifierToggle()
	testHandler.mu.Unlock()

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after debounce timer canceled by regular key, got %v", got)
	}

	// Verify no timers are left dangling.
	if len(testHandler.pendingModifierTimers) != 0 {
		t.Errorf(
			"Expected no pending timers after cancel, got %d",
			len(testHandler.pendingModifierTimers),
		)
	}
}
