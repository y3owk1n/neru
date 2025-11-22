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

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := action.ParseType(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseType() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseType() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("ParseType(%q) = %v, want %v", tt.input, got, tt.want)
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

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.actionType.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
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

	for _, tt := range tests {
		t.Run(tt.actionType.String(), func(t *testing.T) {
			if got := tt.actionType.IsClick(); got != tt.want {
				t.Errorf("IsClick() = %v, want %v", got, tt.want)
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

	for _, tt := range tests {
		t.Run(tt.actionType.String(), func(t *testing.T) {
			if got := tt.actionType.IsMouseButton(); got != tt.want {
				t.Errorf("IsMouseButton() = %v, want %v", got, tt.want)
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
		parsed, err := action.ParseType(str)
		if err != nil {
			t.Errorf("ParseType(%q) error: %v", str, err)
			continue
		}

		if parsed != typ {
			t.Errorf("Round trip failed: %v -> %q -> %v", typ, str, parsed)
		}
	}
}
