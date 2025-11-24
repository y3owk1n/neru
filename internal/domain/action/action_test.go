package action_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
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

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedAction, parsedActionErr := action.ParseType(test.input)

			if test.wantErr {
				if parsedActionErr == nil {
					t.Error("ParseType() expected error, got nil")
				}

				return
			}

			if parsedActionErr != nil {
				t.Errorf("ParseType() unexpected error: %v", parsedActionErr)

				return
			}

			if parsedAction != test.want {
				t.Errorf("ParseType(%q) = %v, want %v", test.input, parsedAction, test.want)
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

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			got := test.actionType.String()
			if got != test.want {
				t.Errorf("String() = %v, want %v", got, test.want)
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

	for _, test := range tests {
		t.Run(test.actionType.String(), func(t *testing.T) {
			got := test.actionType.IsClick()
			if got != test.want {
				t.Errorf("IsClick() = %v, want %v", got, test.want)
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

	for _, test := range tests {
		t.Run(test.actionType.String(), func(t *testing.T) {
			got := test.actionType.IsMouseButton()
			if got != test.want {
				t.Errorf("IsMouseButton() = %v, want %v", got, test.want)
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
