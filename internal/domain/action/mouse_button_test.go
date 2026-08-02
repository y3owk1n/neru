package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

func TestMouseButton_String(t *testing.T) {
	tests := []struct {
		button action.MouseButton
		want   string
	}{
		{action.ButtonLeft, "left"},
		{action.ButtonRight, "right"},
		{action.ButtonMiddle, "middle"},
		{action.MouseButton(99), testUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := testCase.button.String()
			if got != testCase.want {
				t.Errorf("MouseButton.String() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestMousePhase_String(t *testing.T) {
	tests := []struct {
		phase action.MousePhase
		want  string
	}{
		{action.PhaseClick, "click"},
		{action.PhaseDown, "down"},
		{action.PhaseUp, "up"},
		{action.PhaseToggle, "toggle"},
		{action.MousePhase(99), testUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := testCase.phase.String()
			if got != testCase.want {
				t.Errorf("MousePhase.String() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestParsePhase(t *testing.T) {
	tests := []struct {
		input   string
		want    action.MousePhase
		wantErr bool
	}{
		{"down", action.PhaseDown, false},
		{"up", action.PhaseUp, false},
		// click and toggle have their own flags, so they are not --state values.
		{"click", 0, true},
		{"toggle", 0, true},
		{"sideways", 0, true},
		{"", 0, true},
	}

	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			got, err := action.ParsePhase(testCase.input)

			if testCase.wantErr {
				if err == nil {
					t.Errorf("ParsePhase(%q) expected error, got nil", testCase.input)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParsePhase(%q) unexpected error: %v", testCase.input, err)
			}

			if got != testCase.want {
				t.Errorf("ParsePhase(%q) = %v, want %v", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestType_MouseButtonPhase(t *testing.T) {
	tests := []struct {
		actionType action.Type
		wantButton action.MouseButton
		wantPhase  action.MousePhase
		wantOk     bool
	}{
		{action.TypeLeftClick, action.ButtonLeft, action.PhaseClick, true},
		{action.TypeRightClick, action.ButtonRight, action.PhaseClick, true},
		{action.TypeMiddleClick, action.ButtonMiddle, action.PhaseClick, true},
		{action.TypeLeftMouseDown, action.ButtonLeft, action.PhaseDown, true},
		{action.TypeLeftMouseUp, action.ButtonLeft, action.PhaseUp, true},
		{action.TypeRightMouseDown, action.ButtonRight, action.PhaseDown, true},
		{action.TypeRightMouseUp, action.ButtonRight, action.PhaseUp, true},
		{action.TypeMiddleMouseDown, action.ButtonMiddle, action.PhaseDown, true},
		{action.TypeMiddleMouseUp, action.ButtonMiddle, action.PhaseUp, true},
		{action.TypeLeftMouseToggle, action.ButtonLeft, action.PhaseToggle, true},
		{action.TypeRightMouseToggle, action.ButtonRight, action.PhaseToggle, true},
		{action.TypeMiddleMouseToggle, action.ButtonMiddle, action.PhaseToggle, true},
		{action.TypeMoveMouse, 0, 0, false},
		{action.TypeMoveMouseRelative, 0, 0, false},
		{action.TypeScroll, 0, 0, false},
		{action.Type(999), 0, 0, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.actionType.String(), func(t *testing.T) {
			button, phase, ok := testCase.actionType.MouseButtonPhase()

			if ok != testCase.wantOk {
				t.Fatalf("MouseButtonPhase() ok = %v, want %v", ok, testCase.wantOk)
			}

			if !testCase.wantOk {
				return
			}

			if button != testCase.wantButton {
				t.Errorf("MouseButtonPhase() button = %v, want %v", button, testCase.wantButton)
			}

			if phase != testCase.wantPhase {
				t.Errorf("MouseButtonPhase() phase = %v, want %v", phase, testCase.wantPhase)
			}
		})
	}
}

func TestMouseButtonName(t *testing.T) {
	tests := []struct {
		button action.MouseButton
		phase  action.MousePhase
		want   action.Name
	}{
		{action.ButtonLeft, action.PhaseClick, action.NameLeftClick},
		{action.ButtonRight, action.PhaseClick, action.NameRightClick},
		{action.ButtonMiddle, action.PhaseClick, action.NameMiddleClick},
		{action.ButtonLeft, action.PhaseDown, action.NameLeftMouseDown},
		{action.ButtonRight, action.PhaseDown, action.NameRightMouseDown},
		{action.ButtonMiddle, action.PhaseDown, action.NameMiddleMouseDown},
		{action.ButtonLeft, action.PhaseUp, action.NameLeftMouseUp},
		{action.ButtonRight, action.PhaseUp, action.NameRightMouseUp},
		{action.ButtonMiddle, action.PhaseUp, action.NameMiddleMouseUp},
		{action.ButtonLeft, action.PhaseToggle, action.NameLeftMouseToggle},
		{action.ButtonRight, action.PhaseToggle, action.NameRightMouseToggle},
		{action.ButtonMiddle, action.PhaseToggle, action.NameMiddleMouseToggle},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.want), func(t *testing.T) {
			got, ok := action.MouseButtonName(testCase.button, testCase.phase)
			if !ok {
				t.Fatalf("MouseButtonName(%v, %v) not ok", testCase.button, testCase.phase)
			}

			if got != testCase.want {
				t.Errorf("MouseButtonName(%v, %v) = %q, want %q",
					testCase.button, testCase.phase, got, testCase.want)
			}
		})
	}
}

// TestMouseButtonName_RoundTrip pins that every name produced by
// MouseButtonName parses back to a type describing the same button and phase.
func TestMouseButtonName_RoundTrip(t *testing.T) {
	phases := []action.MousePhase{
		action.PhaseClick,
		action.PhaseDown,
		action.PhaseUp,
		action.PhaseToggle,
	}

	for _, button := range action.MouseButtons() {
		for _, phase := range phases {
			name, ok := action.MouseButtonName(button, phase)
			if !ok {
				t.Fatalf("MouseButtonName(%v, %v) not ok", button, phase)
			}

			actionType, err := action.ParseType(string(name))
			if err != nil {
				t.Fatalf("ParseType(%q) error: %v", name, err)
			}

			gotButton, gotPhase, isMouseButton := actionType.MouseButtonPhase()
			if !isMouseButton || gotButton != button || gotPhase != phase {
				t.Errorf("round trip for %q gave (%v, %v, %v), want (%v, %v, true)",
					name, gotButton, gotPhase, isMouseButton, button, phase)
			}
		}
	}
}

func TestMouseButtonName_UnknownPhase(t *testing.T) {
	if _, ok := action.MouseButtonName(action.ButtonLeft, action.MousePhase(99)); ok {
		t.Error("MouseButtonName() with unknown phase should not be ok")
	}

	if _, ok := action.MouseButtonName(action.MouseButton(99), action.PhaseDown); ok {
		t.Error("MouseButtonName() with unknown button should not be ok")
	}
}

// TestLegacyLeftButtonNamesStillParse pins the compatibility promise: configs
// written before right and middle button support keep working.
func TestLegacyLeftButtonNamesStillParse(t *testing.T) {
	tests := []struct {
		name string
		want action.Type
	}{
		{"mouse_down", action.TypeLeftMouseDown},
		{"mouse_up", action.TypeLeftMouseUp},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := action.ParseType(testCase.name)
			if err != nil {
				t.Fatalf("ParseType(%q) error: %v", testCase.name, err)
			}

			if got != testCase.want {
				t.Errorf("ParseType(%q) = %v, want %v", testCase.name, got, testCase.want)
			}

			if !action.IsKnownName(action.Name(testCase.name)) {
				t.Errorf("IsKnownName(%q) = false, want true", testCase.name)
			}

			gotType, typeErr := action.Name(testCase.name).ToType()
			if typeErr != nil || gotType != testCase.want {
				t.Errorf("Name(%q).ToType() = (%v, %v), want (%v, nil)",
					testCase.name, gotType, typeErr, testCase.want)
			}
		})
	}
}
