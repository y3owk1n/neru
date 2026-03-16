//nolint:testpackage // Tests internal function parseModifierEvent
package modes

import (
	"testing"

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

// newTestHandler creates a minimal Handler suitable for testing handleModifierToggle.
// The handler has sticky modifiers enabled and detection already armed.
func newTestHandler() *Handler {
	return &Handler{
		logger:                 zap.NewNop(),
		modifierState:          state.NewModifierState(),
		modifierDetectionArmed: true,
		config: &configpkg.Config{
			StickyModifiers: configpkg.StickyModifiersConfig{
				Enabled: true,
			},
		},
	}
}

func TestHandleModifierToggle_SingleModifier(t *testing.T) {
	testHandler := newTestHandler()
	// Shift↓ → Shift↑ should toggle Shift on.
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")

	if got := testHandler.modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift after single tap, got %v", got)
	}
	// Shift↓ → Shift↑ again should toggle Shift off.
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after double tap, got %v", got)
	}
}

func TestHandleModifierToggle_SimultaneousTwoModifiers(t *testing.T) {
	testHandler := newTestHandler()
	// Simulate pressing Cmd and Shift at the same time:
	// Cmd↓ → Shift↓ → Cmd↑ → Shift↑
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
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
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_alt_up")
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
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_ctrl_down")
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_alt_up")
	testHandler.handleModifierToggle("__modifier_ctrl_up")
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
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_alt_down")
	testHandler.handleModifierToggle("__modifier_alt_up")
	testHandler.handleModifierToggle("__modifier_shift_up")
	testHandler.handleModifierToggle("__modifier_cmd_up")
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
	testHandler.handleModifierToggle("__modifier_cmd_down")
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.cancelPendingModifierToggle() // simulates regular key press
	testHandler.handleModifierToggle("__modifier_cmd_up")
	testHandler.handleModifierToggle("__modifier_shift_up")

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 after regular key canceled pending, got %v", got)
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

func TestHandleModifierToggle_ArmingMechanism(t *testing.T) {
	testHandler := newTestHandler()
	testHandler.modifierDetectionArmed = false
	// Down events while disarmed should be consumed but not toggle.
	testHandler.handleModifierToggle("__modifier_cmd_down")

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 while disarmed, got %v", got)
	}
	// First up event arms detection.
	testHandler.handleModifierToggle("__modifier_cmd_up")

	if !testHandler.modifierDetectionArmed {
		t.Error("Expected detection to be armed after first key-up")
	}

	if got := testHandler.modifierState.Current(); got != 0 {
		t.Errorf("Expected 0 (arming up should not toggle), got %v", got)
	}
	// Now a clean tap should work.
	testHandler.handleModifierToggle("__modifier_shift_down")
	testHandler.handleModifierToggle("__modifier_shift_up")

	if got := testHandler.modifierState.Current(); got != action.ModShift {
		t.Errorf("Expected ModShift after armed tap, got %v", got)
	}
}
