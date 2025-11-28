package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestParseType(t *testing.T) {
	tests := []struct {
		input   string
		want    action.Type
		wantErr bool
	}{
		{"left_click", action.TypeLeftClick, false},
		{"right_click", action.TypeRightClick, false},
		{"middle_click", action.TypeMiddleClick, false},
		{"mouse_down", action.TypeMouseDown, false},
		{"mouse_up", action.TypeMouseUp, false},
		{"move_mouse", action.TypeMoveMouse, false},
		{"scroll", action.TypeScroll, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			parsedAction, parsedActionErr := action.ParseType(testCase.input)

			if testCase.wantErr {
				if parsedActionErr == nil {
					t.Error("ParseType() expected error, got nil")
				}

				return
			}

			if parsedActionErr != nil {
				t.Errorf("ParseType() unexpected error: %v", parsedActionErr)

				return
			}

			if parsedAction != testCase.want {
				t.Errorf("ParseType(%q) = %v, want %v", testCase.input, parsedAction, testCase.want)
			}
		})
	}
}

func TestType_String(t *testing.T) {
	tests := []struct {
		actionType action.Type
		want       string
	}{
		{action.TypeLeftClick, "left_click"},
		{action.TypeRightClick, "right_click"},
		{action.TypeMiddleClick, "middle_click"},
		{action.TypeMouseDown, "mouse_down"},
		{action.TypeMouseUp, "mouse_up"},
		{action.TypeMoveMouse, "move_mouse"},
		{action.TypeScroll, "scroll"},
		{action.Type(999), "unknown"},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := testCase.actionType.String()
			if got != testCase.want {
				t.Errorf("String() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestType_IsClick(t *testing.T) {
	tests := []struct {
		actionType action.Type
		want       bool
	}{
		{action.TypeLeftClick, true},
		{action.TypeRightClick, true},
		{action.TypeMiddleClick, true},
		{action.TypeMouseDown, false},
		{action.TypeMouseUp, false},
		{action.TypeMoveMouse, false},
		{action.TypeScroll, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.actionType.String(), func(t *testing.T) {
			got := testCase.actionType.IsClick()
			if got != testCase.want {
				t.Errorf("IsClick() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestType_IsMouseButton(t *testing.T) {
	tests := []struct {
		actionType action.Type
		want       bool
	}{
		{action.TypeLeftClick, true},
		{action.TypeRightClick, true},
		{action.TypeMiddleClick, true},
		{action.TypeMouseDown, true},
		{action.TypeMouseUp, true},
		{action.TypeMoveMouse, false},
		{action.TypeScroll, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.actionType.String(), func(t *testing.T) {
			got := testCase.actionType.IsMouseButton()
			if got != testCase.want {
				t.Errorf("IsMouseButton() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAllTypes(t *testing.T) {
	types := action.AllTypes()

	if len(types) != 7 {
		t.Errorf("AllTypes() returned %d types, want 7", len(types))
	}

	// Check that all types are unique
	seen := make(map[action.Type]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("AllTypes() contains duplicate: %v", typ)
		}

		seen[typ] = true
	}

	// Check that all expected types are present
	expected := []action.Type{
		action.TypeLeftClick,
		action.TypeRightClick,
		action.TypeMiddleClick,
		action.TypeMouseDown,
		action.TypeMouseUp,
		action.TypeMoveMouse,
		action.TypeScroll,
	}

	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("AllTypes() missing type: %v", exp)
		}
	}
}

func TestParseType_RoundTrip(t *testing.T) {
	// Test that String() and ParseType() are inverses
	for _, typ := range action.AllTypes() {
		str := typ.String()

		parsedType, parsedTypeErr := action.ParseType(str)
		if parsedTypeErr != nil {
			t.Errorf("ParseType(%q) error: %v", str, parsedTypeErr)

			continue
		}

		if parsedType != typ {
			t.Errorf("Round trip failed: %v -> %q -> %v", typ, str, parsedType)
		}
	}
}
