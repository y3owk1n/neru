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
		{"move_mouse_relative", action.TypeMoveMouseRelative, false},
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
		{action.TypeMoveMouseRelative, "move_mouse_relative"},
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

func TestType_IsMoveMouse(t *testing.T) {
	tests := []struct {
		actionType action.Type
		want       bool
	}{
		{action.TypeLeftClick, false},
		{action.TypeRightClick, false},
		{action.TypeMiddleClick, false},
		{action.TypeMouseDown, false},
		{action.TypeMouseUp, false},
		{action.TypeMoveMouse, true},
		{action.TypeMoveMouseRelative, true},
		{action.TypeScroll, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.actionType.String(), func(t *testing.T) {
			got := testCase.actionType.IsMoveMouse()
			if got != testCase.want {
				t.Errorf("IsMoveMouse() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAllTypes(t *testing.T) {
	types := action.AllTypes()

	if len(types) != 8 {
		t.Errorf("AllTypes() returned %d types, want 8", len(types))
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
		action.TypeMoveMouseRelative,
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

func TestName_String(t *testing.T) {
	tests := []struct {
		name action.Name
		want string
	}{
		{action.NameLeftClick, "left_click"},
		{action.NameRightClick, "right_click"},
		{action.NameMiddleClick, "middle_click"},
		{action.NameMouseDown, "mouse_down"},
		{action.NameMouseUp, "mouse_up"},
		{action.NameMoveMouse, "move_mouse"},
		{action.NameMoveMouseRelative, "move_mouse_relative"},
		{action.NameScroll, "scroll"},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := string(testCase.name)
			if got != testCase.want {
				t.Errorf("Name.String() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestIsKnownName(t *testing.T) {
	tests := []struct {
		name action.Name
		want bool
	}{
		{action.NameLeftClick, true},
		{action.NameRightClick, true},
		{action.NameMiddleClick, true},
		{action.NameMouseDown, true},
		{action.NameMouseUp, true},
		{action.NameMoveMouse, true},
		{action.NameMoveMouseRelative, true},
		{action.NameScroll, true},
		{action.Name("unknown"), false},
		{action.Name(""), false},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.name), func(t *testing.T) {
			got := action.IsKnownName(testCase.name)
			if got != testCase.want {
				t.Errorf("IsKnownName(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestKnownNames(t *testing.T) {
	names := action.KnownNames()

	if len(names) != 8 {
		t.Errorf("KnownNames() returned %d names, want 8", len(names))
	}

	// Check that all names are unique
	seen := make(map[action.Name]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("KnownNames() contains duplicate: %v", name)
		}

		seen[name] = true
	}

	// Check that all expected names are present
	expected := []action.Name{
		action.NameLeftClick,
		action.NameRightClick,
		action.NameMiddleClick,
		action.NameMouseDown,
		action.NameMouseUp,
		action.NameMoveMouse,
		action.NameMoveMouseRelative,
		action.NameScroll,
	}

	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("KnownNames() missing name: %v", exp)
		}
	}
}

func TestSupportedNamesString(t *testing.T) {
	str := action.SupportedNamesString()

	// Should contain all action names
	expectedNames := []string{
		"left_click",
		"right_click",
		"middle_click",
		"mouse_down",
		"mouse_up",
		"move_mouse",
		"move_mouse_relative",
		"scroll",
	}

	for _, name := range expectedNames {
		if !contains(str, name) {
			t.Errorf("SupportedNamesString() missing %q in %q", name, str)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func TestType_ToName(t *testing.T) {
	tests := []struct {
		typ  action.Type
		want action.Name
	}{
		{action.TypeLeftClick, action.NameLeftClick},
		{action.TypeRightClick, action.NameRightClick},
		{action.TypeMiddleClick, action.NameMiddleClick},
		{action.TypeMouseDown, action.NameMouseDown},
		{action.TypeMouseUp, action.NameMouseUp},
		{action.TypeMoveMouse, action.NameMoveMouse},
		{action.TypeMoveMouseRelative, action.NameMoveMouseRelative},
		{action.TypeScroll, action.NameScroll},
		{action.Type(999), ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.typ.String(), func(t *testing.T) {
			got := testCase.typ.ToName()
			if got != testCase.want {
				t.Errorf("ToName() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestName_ToType(t *testing.T) {
	tests := []struct {
		name    action.Name
		want    action.Type
		wantErr bool
	}{
		{action.NameLeftClick, action.TypeLeftClick, false},
		{action.NameRightClick, action.TypeRightClick, false},
		{action.NameMiddleClick, action.TypeMiddleClick, false},
		{action.NameMouseDown, action.TypeMouseDown, false},
		{action.NameMouseUp, action.TypeMouseUp, false},
		{action.NameMoveMouse, action.TypeMoveMouse, false},
		{action.NameMoveMouseRelative, action.TypeMoveMouseRelative, false},
		{action.NameScroll, action.TypeScroll, false},
		{action.Name("unknown"), 0, true},
		{action.Name(""), 0, true},
	}

	for _, testCase := range tests {
		t.Run(string(testCase.name), func(t *testing.T) {
			got, err := testCase.name.ToType()

			if testCase.wantErr {
				if err == nil {
					t.Errorf("ToType() expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("ToType() unexpected error: %v", err)

				return
			}

			if got != testCase.want {
				t.Errorf("ToType() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestType_Name_RoundTrip(t *testing.T) {
	// Test that ToName() and ToType() are inverses for all valid types
	for _, typ := range action.AllTypes() {
		name := typ.ToName()

		parsedType, err := name.ToType()
		if err != nil {
			t.Errorf("Round trip error for %v: %v", typ, err)

			continue
		}

		if parsedType != typ {
			t.Errorf("Round trip failed: %v -> %v -> %v", typ, name, parsedType)
		}
	}
}

func TestName_Type_ParseType_RoundTrip(t *testing.T) {
	// Test that ParseType() and String() work with Name conversion
	for _, name := range action.KnownNames() {
		// Name -> Type via ToType
		typ, err := name.ToType()
		if err != nil {
			t.Errorf("ToType(%v) error: %v", name, err)

			continue
		}

		// Type -> string via String()
		str := typ.String()

		// string -> Type via ParseType
		parsedType, err := action.ParseType(str)
		if err != nil {
			t.Errorf("ParseType(%q) error: %v", str, err)

			continue
		}

		// Should get back the same type
		if parsedType != typ {
			t.Errorf("Round trip failed: %v -> %v -> %q -> %v", name, typ, str, parsedType)
		}
	}
}
